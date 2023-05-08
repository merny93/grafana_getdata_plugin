package plugin

//stupid stupid need to run go clean -cache cause or else its dumbbbb

/*
#cgo LDFLAGS: -L/usr/local/lib -lgetdata
#include <getdata.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
)

type GD_dirfile *C.DIRFILE
type Dirfile struct {
	df    *C.DIRFILE
	mutex *sync.Mutex
}

func GD_open(dir_file_name string) Dirfile {
	//open a dirfile
	//this is unsafe, does not check success
	var df *C.DIRFILE
	file_name_c := C.CString(dir_file_name)
	df = C.gd_open(file_name_c, C.GD_RDONLY)
	C.free(unsafe.Pointer(file_name_c))
	return Dirfile{df: df, mutex: &sync.Mutex{}}
}

func GD_getdata(field_name string, df Dirfile, first_frame, first_sample, num_frames, num_samples int) []float64 {

	defer df.mutex.Unlock()
	df.mutex.Lock()

	// convert the field name to c string
	field_name_c := C.CString(field_name)

	//make dummy result array pointer
	var dummy_result *float64
	num_elements := C.gd_getdata(df.df, field_name_c, C.long(first_frame), C.long(first_sample), C.ulong(num_frames), C.ulong(num_samples), C.GD_NULL, unsafe.Pointer(dummy_result))

	if num_elements == 0 {
		fmt.Println("Error: field not found")
		return nil
	}
	//make result array
	result := make([]float64, num_elements)
	//pass the result array as a pointer using the first element of result assuming its contiguous
	C.gd_getdata(df.df, field_name_c, C.long(first_frame), C.long(first_sample), C.ulong(num_frames), C.ulong(num_samples), C.GD_FLOAT64, unsafe.Pointer(&result[0]))

	C.free(unsafe.Pointer(field_name_c))
	return result
}

func GD_close(df Dirfile) {
	C.gd_close(df.df)
}

func GD_framenum(df Dirfile, field_name string, value float64) float64 {

	defer df.mutex.Unlock()
	df.mutex.Lock()

	field_name_c := C.CString(field_name)
	var index float64
	index = float64(C.gd_framenum(df.df, field_name_c, C.double(value)))
	C.free(unsafe.Pointer(field_name_c))
	return index
}
