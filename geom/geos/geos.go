package geos

/*
#cgo LDFLAGS: -lgeos_c
#include "geos_c.h"
#include <stdlib.h>

extern void goLogString(char *msg);
extern void debug_wrap(const char *fmt, ...);
extern GEOSContextHandle_t initGEOS_r_debug();
extern void initGEOS_debug();
*/
import "C"

import (
	"errors"
	"runtime"
	"unsafe"

	"github.com/naturalatlas/imposm3/log"
)

//export goLogString
func goLogString(msg *C.char) {
	log.Printf(C.GoString(msg))
}

type Geos struct {
	v         C.GEOSContextHandle_t
	srid      int
	wkbwriter *C.GEOSWKBWriter
}

type Geom struct {
	v *C.GEOSGeometry
}

type CreateError string
type Error string

func (e Error) Error() string {
	return string(e)
}

func (e CreateError) Error() string {
	return string(e)
}

func NewGeos() *Geos {
	geos := &Geos{}
	geos.v = C.initGEOS_r_debug()
	return geos
}

func (g *Geos) Finish() {
	if g.v != nil {
		C.finishGEOS_r(g.v)
		g.v = nil
	}
}

func init() {
	/*
		Init global GEOS handle for non _r calls.
		In theory we need to always call the _r functions
		with a thread/goroutine-local GEOS instance to get thread
		safe behaviour. Some functions don't need a GEOS instance though
		and we can make use of that e.g. to call GEOSGeom_destroy in
		finalizer.
	*/
	C.initGEOS_debug()
}

func (g *Geos) Destroy(geom *Geom) {
	runtime.SetFinalizer(geom, nil)
	if geom.v != nil {
		C.GEOSGeom_destroy_r(g.v, geom.v)
		geom.v = nil
	} else {
		log.Printf("double free?")
	}
}

func destroyGeom(geom *Geom) {
	C.GEOSGeom_destroy(geom.v)
}

func (g *Geos) DestroyLater(geom *Geom) {
	runtime.SetFinalizer(geom, destroyGeom)
}

func (g *Geos) Clone(geom *Geom) *Geom {
	if geom == nil || geom.v == nil {
		return nil
	}

	result := C.GEOSGeom_clone_r(g.v, geom.v)
	if result == nil {
		return nil
	}
	return &Geom{result}
}

func (g *Geos) SetHandleSrid(srid int) {
	g.srid = srid
}

func (g *Geos) NumGeoms(geom *Geom) int32 {
	count := int32(C.GEOSGetNumGeometries_r(g.v, geom.v))
	return count
}

func (g *Geos) NumCoordinates(geom *Geom) int32 {
	count := int32(C.GEOSGetNumCoordinates_r(g.v, geom.v))
	return count
}

func (g *Geos) Geoms(geom *Geom) []*Geom {
	count := g.NumGeoms(geom)
	var result []*Geom
	for i := 0; int32(i) < count; i++ {
		part := C.GEOSGetGeometryN_r(g.v, geom.v, C.int(i))
		if part == nil {
			return nil
		}
		result = append(result, &Geom{part})
	}
	return result
}

func (g *Geos) ExteriorRing(geom *Geom) *Geom {
	ring := C.GEOSGetExteriorRing_r(g.v, geom.v)
	if ring == nil {
		return nil
	}
	return &Geom{ring}
}

func (g *Geos) BoundsPolygon(bounds Bounds) *Geom {
	coordSeq, err := g.CreateCoordSeq(5, 2)
	if err != nil {
		return nil
	}
	// coordSeq inherited by LineString, no destroy

	if err := coordSeq.SetXY(g, 0, bounds.MinX, bounds.MinY); err != nil {
		return nil
	}
	if err := coordSeq.SetXY(g, 1, bounds.MaxX, bounds.MinY); err != nil {
		return nil
	}
	if err := coordSeq.SetXY(g, 2, bounds.MaxX, bounds.MaxY); err != nil {
		return nil
	}
	if err := coordSeq.SetXY(g, 3, bounds.MinX, bounds.MaxY); err != nil {
		return nil
	}
	if err := coordSeq.SetXY(g, 4, bounds.MinX, bounds.MinY); err != nil {
		return nil
	}

	geom, err := coordSeq.AsLinearRing(g)
	if err != nil {
		return nil
	}
	// geom inherited by Polygon, no destroy

	geom = g.Polygon(geom, nil)
	return geom

}

func (g *Geos) Point(x, y float64) *Geom {
	coordSeq, err := g.CreateCoordSeq(1, 2)
	if err != nil {
		return nil
	}
	// coordSeq inherited by LineString
	coordSeq.SetXY(g, 0, x, y)
	geom, err := coordSeq.AsPoint(g)
	if err != nil {
		return nil
	}
	return geom
}

func (g *Geos) Polygon(exterior *Geom, interiors []*Geom) *Geom {
	if len(interiors) == 0 {
		geom := C.GEOSGeom_createPolygon_r(g.v, exterior.v, nil, C.uint(0))
		if geom == nil {
			return nil
		}
		err := C.GEOSNormalize_r(g.v, geom)
		if err != 0 {
			C.GEOSGeom_destroy(geom)
			return nil
		}
		return &Geom{geom}
	}

	interiorPtr := make([]*C.GEOSGeometry, len(interiors))
	for i, geom := range interiors {
		interiorPtr[i] = geom.v
	}
	geom := C.GEOSGeom_createPolygon_r(g.v, exterior.v, &interiorPtr[0], C.uint(len(interiors)))
	if geom == nil {
		return nil
	}
	err := C.GEOSNormalize_r(g.v, geom)
	if err != 0 {
		C.GEOSGeom_destroy(geom)
		return nil
	}
	return &Geom{geom}
}

func (g *Geos) MultiPolygon(polygons []*Geom) *Geom {
	if len(polygons) == 0 {
		return nil
	}
	polygonPtr := make([]*C.GEOSGeometry, len(polygons))
	for i, geom := range polygons {
		polygonPtr[i] = geom.v
	}
	geom := C.GEOSGeom_createCollection_r(g.v, C.GEOS_MULTIPOLYGON, &polygonPtr[0], C.uint(len(polygons)))
	if geom == nil {
		return nil
	}
	return &Geom{geom}
}
func (g *Geos) MultiLineString(lines []*Geom) *Geom {
	if len(lines) == 0 {
		return nil
	}
	linePtr := make([]*C.GEOSGeometry, len(lines))
	for i, geom := range lines {
		linePtr[i] = geom.v
	}
	geom := C.GEOSGeom_createCollection_r(g.v, C.GEOS_MULTILINESTRING, &linePtr[0], C.uint(len(lines)))
	if geom == nil {
		return nil
	}
	return &Geom{geom}
}

func (g *Geos) IsValid(geom *Geom) bool {
	if C.GEOSisValid_r(g.v, geom.v) == 1 {
		return true
	}
	return false
}

func (g *Geos) IsSimple(geom *Geom) bool {
	if C.GEOSisSimple_r(g.v, geom.v) == 1 {
		return true
	}
	return false
}

func (g *Geos) IsEmpty(geom *Geom) bool {
	if C.GEOSisEmpty_r(g.v, geom.v) == 1 {
		return true
	}
	return false
}

func (g *Geos) Type(geom *Geom) string {
	geomType := C.GEOSGeomType_r(g.v, geom.v)
	if geomType == nil {
		return "Unknown"
	}
	defer C.free(unsafe.Pointer(geomType))
	return C.GoString(geomType)
}

func (g *Geos) Equals(a, b *Geom) bool {
	result := C.GEOSEquals_r(g.v, a.v, b.v)
	if result == 1 {
		return true
	}
	return false
}

func (g *Geos) MakeValid(geom *Geom) (*Geom, error) {
	if g.IsValid(geom) {
		return geom, nil
	}
	fixed := g.Buffer(geom, 0)
	if fixed == nil {
		return nil, errors.New("Error while fixing geom with buffer(0)")
	}
	g.Destroy(geom)

	return fixed, nil
}

func (g *Geom) Area() float64 {
	var area C.double
	if ret := C.GEOSArea(g.v, &area); ret == 1 {
		return float64(area)
	}
	return 0
}

func (g *Geom) Length() float64 {
	var length C.double
	if ret := C.GEOSLength(g.v, &length); ret == 1 {
		return float64(length)
	}
	return 0
}

type Bounds struct {
	MinX float64
	MinY float64
	MaxX float64
	MaxY float64
}

func MakeBounds(minx, miny, maxx, maxy float64) Bounds {
	return Bounds{
		MinX: minx,
		MinY: miny,
		MaxX: maxx,
		MaxY: maxy,
	}
}

var NilBounds = Bounds{1e20, 1e20, -1e20, -1e20}

func (g *Geom) Bounds() Bounds {
	geom := C.GEOSEnvelope(g.v)
	if geom == nil {
		return NilBounds
	}
	defer C.GEOSGeom_destroy(geom)
	extRing := C.GEOSGetExteriorRing(geom)
	if extRing == nil {
		return NilBounds
	}
	cs := C.GEOSGeom_getCoordSeq(extRing)
	var csLen C.uint
	C.GEOSCoordSeq_getSize(cs, &csLen)
	minx := 1.e+20
	maxx := -1e+20
	miny := 1.e+20
	maxy := -1e+20
	var temp C.double
	for i := 0; i < int(csLen); i++ {
		C.GEOSCoordSeq_getX(cs, C.uint(i), &temp)
		x := float64(temp)
		if x < minx {
			minx = x
		}
		if x > maxx {
			maxx = x
		}
		C.GEOSCoordSeq_getY(cs, C.uint(i), &temp)
		y := float64(temp)
		if y < miny {
			miny = y
		}
		if y > maxy {
			maxy = y
		}
	}

	return Bounds{minx, miny, maxx, maxy}
}
