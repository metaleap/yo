package yoctx

import (
	"reflect"
	"sync"
)

func (me *Ctx) cacheEnsure(typeIdent string) (cacheMap map[any]any, cacheMut *sync.RWMutex) {
	me.caches.mut.Lock()
	defer me.caches.mut.Unlock()
	cacheMap, cacheMut = me.caches.maps[typeIdent], me.caches.muts[typeIdent]
	if cacheMap == nil {
		cacheMap = map[any]any{}
		me.caches.maps[typeIdent] = cacheMap
	}
	if cacheMut == nil {
		cacheMut = new(sync.RWMutex)
		me.caches.muts[typeIdent] = cacheMut
	}
	return
}

func Cache[TKey comparable, TValue any](ctx *Ctx, key TKey, new func() TValue) (ret TValue) {
	if ctx == nil {
		return new()
	}
	cache, mut := ctx.cacheEnsure(reflect.TypeOf(ret).String())
	mut.RLock()
	cached, found := cache[key]
	mut.RUnlock()
	if !found {
		cached = new()
		mut.Lock()
		cache[key] = cached
		mut.Unlock()
	}
	ret = cached.(TValue)
	return
}
