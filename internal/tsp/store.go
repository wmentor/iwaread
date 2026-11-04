package tsp

import (
	"github.com/wmentor/iwaread/internal/base"
)

// ObjectStore indexes all IWA objects by identifier.
type ObjectStore struct {
	objects map[uint64]*IwaObject
}

// NewObjectStore creates an ObjectStore from a container.
func NewObjectStore(container *IWorkContainer) (*ObjectStore, error) {
	store := &ObjectStore{
		objects: make(map[uint64]*IwaObject),
	}
	for _, iwaData := range container.IwaFiles {
		objects, err := ParseIwaFile(iwaData)
		if err != nil {
			// Skip components that fail to parse (e.g., LZFSE-compressed)
			continue
		}
		for _, obj := range objects {
			store.objects[obj.Identifier] = obj
		}
	}
	return store, nil
}

// Object returns the object with the given identifier.
func (s *ObjectStore) Object(id uint64) *IwaObject {
	return s.objects[id]
}

// FindByType returns the first object of the given type.
func (s *ObjectStore) FindByType(typeID int) *IwaObject {
	for _, obj := range s.objects {
		if obj.Type == typeID {
			return obj
		}
	}
	return nil
}

// AllObjects returns all objects in the store.
func (s *ObjectStore) AllObjects() []*IwaObject {
	result := make([]*IwaObject, 0, len(s.objects))
	for _, obj := range s.objects {
		result = append(result, obj)
	}
	return result
}

// Resolve resolves a TSP.Reference message to its object.
func (s *ObjectStore) Resolve(msg *base.RawMessage) *IwaObject {
	if msg == nil {
		return nil
	}
	id, ok := msg.GetVarint(1)
	if !ok {
		return nil
	}
	return s.Object(id)
}

// RefID extracts the identifier from a TSP.Reference field.
func RefID(container *base.RawMessage, fieldNo int) (uint64, bool) {
	if container == nil {
		return 0, false
	}
	sub, err := container.GetMessage(fieldNo)
	if err != nil || sub == nil {
		return 0, false
	}
	return sub.GetVarint(1)
}
