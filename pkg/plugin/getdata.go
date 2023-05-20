package plugin

//stupid stupid need to run go clean -cache cause or else its dumbbbb

/*
#cgo LDFLAGS: -L/usr/local/lib -lgetdata
#include <getdata.h>
#include <stdlib.h>
*/
import "C"

import (
	"errors"
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
	defer C.free(unsafe.Pointer(file_name_c))
	df = C.gd_open(file_name_c, C.GD_RDONLY)
	return Dirfile{df: df, mutex: &sync.Mutex{}}
}

func GD_getdata(field_name string, df Dirfile, first_frame, num_frames int) ([]float64, error) {
	//i got rid of sample calles cause i dont think we need them and not sure what to do with them anyways...
	//same as GD_getdata_ but gets the size by computing the size from spf and nframes
	//also it does not need the mutex cause the subcalls have it

	if num_frames <= 0 {
		//this is weird, lets not think about it
		return nil, errors.New("num_frames must be greater than 0")
	}

	spf := GD_spf(df, field_name)
	nframes := GD_nframes(df)

	//if the first frame is out of bounds, return nil
	if first_frame >= nframes-1 {
		return nil, errors.New("first_frame is out of bounds")
	}

	//get number of frames to read
	if first_frame+num_frames > nframes {
		num_frames = nframes - first_frame
	}

	//allocate the result array
	res := make([]float64, num_frames*spf)

	// convert the field name to c string
	field_name_c := C.CString(field_name)
	defer C.free(unsafe.Pointer(field_name_c))

	df.mutex.Lock()
	C.gd_getdata(df.df, field_name_c, C.long(first_frame), 0, C.ulong(num_frames), 0, C.GD_FLOAT64, unsafe.Pointer(&res[0]))
	df.mutex.Unlock()

	err := GD_error(df)

	return res, err
}

func GD_getdata_c(field_name string, df Dirfile, first_frame, first_sample, num_frames, num_samples int, result []float64) int {
	//leave the responsability of allocating the result array to the caller

	defer df.mutex.Unlock()
	df.mutex.Lock()

	// convert the field name to c string
	field_name_c := C.CString(field_name)
	defer C.free(unsafe.Pointer(field_name_c))

	//pass the result array as a pointer using the first element of result assuming its contiguous
	numSamples := C.gd_getdata(df.df, field_name_c, C.long(first_frame), C.long(first_sample), C.ulong(num_frames), C.ulong(num_samples), C.GD_FLOAT64, unsafe.Pointer(&result[0]))

	return int(numSamples)
}

func GD_close(df Dirfile) {
	C.gd_close(df.df)
}

func GD_framenum(df Dirfile, field_name string, value float64) float64 {

	defer df.mutex.Unlock()
	df.mutex.Lock()

	field_name_c := C.CString(field_name)
	defer C.free(unsafe.Pointer(field_name_c))

	var index float64

	index = float64(C.gd_framenum(df.df, field_name_c, C.double(value)))

	return index
}

func GD_match_entries(df Dirfile, regexString string) []string {
	//returns a list of all entries in the dirfile
	//that match the regex string

	defer (df.mutex).Unlock()
	(df.mutex).Lock()

	regexString_c := C.CString(regexString)
	defer C.free(unsafe.Pointer(regexString_c))

	var result **C.char

	numMatches := C.gd_match_entries(df.df, regexString_c, 0, 0, C.GD_REGEX_CASELESS, &result)

	//loop through the result and convert to go string
	result_go := make([]string, numMatches)
	for i := 0; i < int(numMatches); i++ {
		//chatgpt said this was gonna work and it was right, shocking
		// this is doing pointer arithmetic to get the ith element of the result array, here Sizeof(result) should just be a pointer size
		// then we dereference that pointer to get the string
		result_go[i] = C.GoString(*(**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(result)) + uintptr(i)*unsafe.Sizeof(result))))
	}

	//do not free the result as it is managed by the getdata library and will be freeed when the dirfile is closed

	return result_go

}

func GD_error(df Dirfile) error {

	defer (df.mutex).Unlock()
	(df.mutex).Lock()

	if C.gd_error(df.df) == 0 {
		return nil
	}

	errorSringPointer := C.gd_error_string(df.df, nil, 0)

	errorStringGo := C.GoString(errorSringPointer)

	//from the GetData docs for gd_error_string:
	// If buffer is NULL, a pointer to a newly-allocated buffer containing the entire error string is returned.
	// In this case, buflen is ignored. This string will be allocated on the caller's heap and should be deallocated by the caller when no longer needed.
	C.free(unsafe.Pointer(errorSringPointer))

	return errors.New(errorStringGo)
}

func GD_nframes(df Dirfile) int {
	defer (df.mutex).Unlock()
	(df.mutex).Lock()

	return int(C.gd_nframes(df.df))
}

func GD_spf(df Dirfile, fieldName string) int {
	defer (df.mutex).Unlock()
	(df.mutex).Lock()

	fieldName_c := C.CString(fieldName)
	defer C.free(unsafe.Pointer(fieldName_c))

	return int(C.gd_spf(df.df, fieldName_c))
}
