package cache

import (
	"sync"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal/async"
)

var (
	// textTextures is looked up for every visible canvas.Text on every frame,
	// so it is a typed map: async.Map would box the FontCacheEntry key, an
	// allocation per lookup.
	textTexturesLock sync.Mutex
	textTextures     = map[FontCacheEntry]*textureInfo{}

	objectTextures async.Map[fyne.CanvasObject, *textureInfo]
)

// DeleteTexture deletes the texture from the cache map.
func DeleteTexture(obj fyne.CanvasObject) {
	objectTextures.Delete(obj)
}

// GetTextTexture gets cached texture for a text run.
func GetTextTexture(ent FontCacheEntry) (TextureType, bool) {
	textTexturesLock.Lock()
	texInfo, ok := textTextures[ent]
	textTexturesLock.Unlock()
	if texInfo == nil || !ok {
		return NoTexture, false
	}
	texInfo.setAlive()
	return texInfo.texture, true
}

// GetTexture gets cached texture.
func GetTexture(obj fyne.CanvasObject) (TextureType, bool) {
	texInfo, ok := objectTextures.Load(obj)
	if texInfo == nil || !ok {
		return NoTexture, false
	}
	texInfo.setAlive()
	return texInfo.texture, true
}

// RangeExpiredTexturesFor range over the expired textures for the specified canvas.
//
// Note: If this is used to free textures, then it should be called inside a current
// gl context to ensure textures are deleted from gl.
func RangeExpiredTexturesFor(canvas fyne.Canvas, f func(fyne.CanvasObject)) {
	now := timeNow()

	textTexturesLock.Lock()
	for key, tinfo := range textTextures {
		// Just free text directly when that string/style combo is done.
		if tinfo.isExpired(now) && tinfo.canvas == canvas {
			delete(textTextures, key)
			tinfo.textFree()
		}
	}
	textTexturesLock.Unlock()

	objectTextures.Range(func(obj fyne.CanvasObject, tinfo *textureInfo) bool {
		if tinfo.isExpired(now) && tinfo.canvas == canvas {
			f(obj)
		}
		return true
	})
}

// RangeTexturesFor range over the textures for the specified canvas.
// It will not return the texture for a `canvas.Text` as their render lifecycle is handled separately.
//
// Note: If this is used to free textures, then it should be called inside a current
// gl context to ensure textures are deleted from gl.
func RangeTexturesFor(canvas fyne.Canvas, f func(fyne.CanvasObject)) {
	// Do nothing for texture cache, it lives outside the scope of an object.
	objectTextures.Range(func(obj fyne.CanvasObject, tinfo *textureInfo) bool {
		if tinfo.canvas == canvas {
			f(obj)
		}
		return true
	})
}

// DeleteTextTexturesFor deletes all text textures for the given canvas.
func DeleteTextTexturesFor(canvas fyne.Canvas) {
	textTexturesLock.Lock()
	for key, tinfo := range textTextures {
		if tinfo.canvas == canvas {
			delete(textTextures, key)
			tinfo.textFree()
		}
	}
	textTexturesLock.Unlock()
}

// SetTextTexture sets cached texture for a text run.
func SetTextTexture(ent FontCacheEntry, texture TextureType, canvas fyne.Canvas, free func()) {
	tinfo := prepareTexture(texture, canvas, free)
	textTexturesLock.Lock()
	textTextures[ent] = tinfo
	textTexturesLock.Unlock()
}

// SetTexture sets cached texture.
func SetTexture(obj fyne.CanvasObject, texture TextureType, canvas fyne.Canvas) {
	tinfo := prepareTexture(texture, canvas, nil)
	objectTextures.Store(obj, tinfo)
}

func prepareTexture(texture TextureType, canvas fyne.Canvas, free func()) *textureInfo {
	tinfo := &textureInfo{texture: texture, textFree: free}
	tinfo.canvas = canvas
	tinfo.setAlive()
	return tinfo
}

// textureCacheBase defines base texture cache object.
type textureCacheBase struct {
	expiringCache
	canvas fyne.Canvas
}
