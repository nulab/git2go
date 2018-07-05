package git

/*
#include <git2.h>
#include <git2/sys/mempack.h>

extern int git_mempack_new(git_odb_backend **out);
extern int git_mempack_dump(git_buf *pack, git_repository *repo, git_odb_backend *backend);
extern void git_mempack_reset(git_odb_backend *backend);
extern void _go_git_odb_backend_free(git_odb_backend *backend);
*/
import "C"

import (
	"runtime"
	"unsafe"
)

// Mempack is a custom ODB backend that permits packing object in-memory.
type Mempack struct {
	ptr *C.git_odb_backend
}

// NewMempack creates a new mempack instance and registers it to the ODB.
func NewMempack(odb *Odb) (mempack *Mempack, err error) {
	mempack = new(Mempack)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ret := C.git_mempack_new(&mempack.ptr)
	if ret < 0 {
		return nil, MakeGitError(ret)
	}
	runtime.SetFinalizer(mempack, (*Mempack).Free)

	ret = C.git_odb_add_backend(odb.ptr, mempack.ptr, C.int(999))
	if ret < 0 {
		// Since git_odb_add_alternate() takes ownership of the ODB backend, the
		// only case in which we free the mempack's memory is if it fails to be
		// added to the ODB. Mempack.Free() is actually just a no-op.
		C._go_git_odb_backend_free(mempack.ptr)
		mempack.Free()
		return nil, MakeGitError(ret)
	}

	return mempack, nil
}

// Dump dumps all the queued in-memory writes to a packfile.
//
// It is the caller's responsibility to ensure that the generated packfile is
// available to the repository (e.g. by writing it to disk, or doing something
// crazy like distributing it across several copies of the repository over a
// network).
//
// Once the generated packfile is available to the repository, call
// Mempack.Reset to cleanup the memory store.
//
// Calling Mempack.Reset before the packfile has been written to disk will
// result in an inconsistent repository (the objects in the memory store won't
// be accessible).
func (mempack *Mempack) Dump(repository *Repository) ([]byte, error) {
	buf := C.git_buf{}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var repoPtr *C.git_repository
	if repository != nil {
		repoPtr = repository.ptr
	}

	ret := C.git_mempack_dump(&buf, repoPtr, mempack.ptr)
	if ret < 0 {
		return nil, MakeGitError(ret)
	}
	defer C.git_buf_free(&buf)

	return C.GoBytes(unsafe.Pointer(buf.ptr), C.int(buf.size)), nil
}

// Reset resets the memory packer by clearing all the queued objects.
//
// This assumes that Mempack.Dump has been called before to store all the
// queued objects into a single packfile.
func (mempack *Mempack) Reset() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	C.git_mempack_reset(mempack.ptr)
}

// Free frees the mempack and its resources.
func (mempack *Mempack) Free() {
	runtime.SetFinalizer(mempack, nil)
}
