package cache

import (
	"github.com/jmhodges/levigo"
	osm "github.com/omniscale/go-osm"
	"github.com/naturalatlas/imposm3/cache/binary"
)

type RelationsCache struct {
	cache
}

func newRelationsCache(path string) (*RelationsCache, error) {
	cache := RelationsCache{}
	cache.options = &globalCacheOptions.Relations
	err := cache.open(path)
	if err != nil {
		return nil, err
	}
	return &cache, err
}

func (p *RelationsCache) PutRelation(relation *osm.Relation) error {
	if relation.ID == SKIP {
		return nil
	}
	keyBuf := idToKeyBuf(relation.ID)
	data, err := binary.MarshalRelation(relation)
	if err != nil {
		return err
	}
	return p.db.Put(p.wo, keyBuf, data)
}

func (p *RelationsCache) PutRelations(rels []osm.Relation) error {
	batch := levigo.NewWriteBatch()
	defer batch.Close()

	for _, rel := range rels {
		if rel.ID == SKIP {
			continue
		}
		if len(rel.Tags) == 0 {
			continue
		}
		keyBuf := idToKeyBuf(rel.ID)
		data, err := binary.MarshalRelation(&rel)
		if err != nil {
			return err
		}
		batch.Put(keyBuf, data)
	}
	return p.db.Write(p.wo, batch)
}

func (p *RelationsCache) Iter() chan *osm.Relation {
	rels := make(chan *osm.Relation)
	go func() {
		ro := levigo.NewReadOptions()
		ro.SetFillCache(false)
		it := p.db.NewIterator(ro)
		// we need to Close the iter before closing the
		// chan (and thus signaling that we are done)
		// to avoid race where db is closed before the iterator
		defer close(rels)
		defer it.Close()
		it.SeekToFirst()
		for ; it.Valid(); it.Next() {
			rel, err := binary.UnmarshalRelation(it.Value())
			if err != nil {
				panic(err)
			}
			rel.ID = idFromKeyBuf(it.Key())

			rels <- rel
		}
	}()
	return rels
}

func (p *RelationsCache) GetRelation(id int64) (*osm.Relation, error) {
	keyBuf := idToKeyBuf(id)
	data, err := p.db.Get(p.ro, keyBuf)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, NotFound
	}
	relation, err := binary.UnmarshalRelation(data)
	if err != nil {
		return nil, err
	}
	relation.ID = id
	return relation, err
}

func (p *RelationsCache) DeleteRelation(id int64) error {
	keyBuf := idToKeyBuf(id)
	return p.db.Delete(p.wo, keyBuf)
}
