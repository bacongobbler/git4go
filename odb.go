package git4go

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	GIT_LOOSE_PRIORITY       = 1
	GIT_PACKED_PRIORITY      = 2
	GIT_ALTERNATES_MAX_DEPTH = 5
	GIT_ALTERNATES_FILE      = "info/alternates"
)

func (r *Repository) Odb() (odb *Odb, err error) {
	if r.odb == nil {
		r.odb = &Odb{}
	}
	return r.odb, nil
}

// Odb type and its methods

type Odb struct {
	backends []OdbBackend
}

func OdbOpen(objectsDir string) (*Odb, error) {
	odb := &Odb{}
	err := odb.AddDefaultBackends(objectsDir, false, 0)
	return odb, err
}

func (o *Odb) AddDefaultBackends(objectsDir string, asAlternates bool, alternateDepth int) error {
	info, err := os.Stat(objectsDir)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to load object database in '%s'", objectsDir))
	}
	for _, backend := range o.backends {
		if backend.SameDirectory(info) {
			return nil
		}
	}
	loose := NewOdbBackendLoose(objectsDir, -1, false, 0, 0)
	o.addBackendInternal(loose, GIT_LOOSE_PRIORITY, asAlternates, info)
	//packed := &OdbBackendLoose{}
	//o.addBackendInternal(packed, GIT_PACKED_PRIORITY, asAlternates)
	o.loadAlternates(objectsDir, alternateDepth)
	return nil
}

func (o *Odb) Exists(oid *Oid) bool {
	for _, backend := range o.backends {
		if backend.Exists(oid) {
			return true
		}
	}
	return false
}

func (o *Odb) ExistsPrefix(oid *Oid, length int) (*Oid, error) {
	var foundId *Oid
	var err error
	for _, backend := range o.backends {
		foundId, err = backend.ExistsPrefix(oid, length)
		if foundId != nil {
			return foundId, nil
		}
	}
	return nil, err
}

func (o *Odb) Read(oid *Oid) (*OdbObject, error) {
	for _, backend := range o.backends {
		odbObject, err := backend.Read(oid)
		if err == nil {
			return odbObject, nil
		}
	}

	return nil, errors.New(fmt.Sprintf("Odb.Read: no match for id: %s", oid.String()))
}

func (o *Odb) ReadPrefix(oid *Oid, length int) (*Oid, *OdbObject, error) {
	var foundId *Oid
	var foundObject *OdbObject
	var err error

	for _, backend := range o.backends {
		foundId, foundObject, err = backend.ReadPrefix(oid, length)
		if err == nil {
			return foundId, foundObject, nil
		}
	}

	return nil, nil, errors.New(fmt.Sprintf("Odb.ReadPrefix: no match for id: %s", oid.String()))
}

func (o *Odb) ReadHeader(oid *Oid) (ObjectType, int64, error) {
	for _, backend := range o.backends {
		objType, size, err := backend.ReadHeader(oid)
		if err == nil {
			return objType, size, nil
		}
	}

	return ObjectBad, -1, errors.New(fmt.Sprintf("Odb.ReadHeader: no match for id: %s", oid.String()))
}

// internal functions and methods

func (o *Odb) addBackendInternal(backend OdbBackend, priority int, asAlternates bool, dirInfo os.FileInfo) {
	backend.InitBackend(priority, asAlternates, dirInfo)
	o.backends = append(o.backends, backend)
	var backends OdbBackends = o.backends
	sort.Sort(backends)
}

func (o *Odb) loadAlternates(objectsDir string, alternateDepth int) error {
	if alternateDepth > GIT_ALTERNATES_MAX_DEPTH {
		return nil
	}
	alternatePath := filepath.Join(objectsDir, GIT_ALTERNATES_FILE)
	_, err := os.Stat(alternatePath)
	if os.IsNotExist(err) {
		return nil
	}
	file, err := os.Open(alternatePath)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		if line[0] == '.' && alternateDepth > 0 {
			alternatesPath2 := filepath.Join(objectsDir, line)
			err = o.AddDefaultBackends(alternatesPath2, true, alternateDepth+1)
			if err != nil {
				return err
			}
		}
	}
	return nil
}