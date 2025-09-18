package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// A helper writer for tests to simulate writing errors.
type errorWriter struct{}

// A helper function for tests to simulate writing failure.
func (errorWriter) Write(p []byte) (int, error) {
	return 0, errors.New("simulated write failure")
}

// Expectation: The tar buffer should contain the appropriate files and folders.
func Test_writeDummyFile_Success(t *testing.T) {
	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)
	require.NotNil(t, tw)

	require.NoError(t, writeDummyFile(tw, "foo.txt", false))
	require.NoError(t, writeDummyFile(tw, "bar", true))
	require.NoError(t, tw.Close())

	tr := tar.NewReader(&buf)
	require.NotNil(t, tr)

	var names []string
	for {
		hdr, err := tr.Next()

		if err == io.EOF {
			break
		}

		require.NoError(t, err)
		names = append(names, hdr.Name)
	}

	require.Equal(t, []string{"foo.txt", "bar/"}, names)
}

// Expectation: The function should return the correct error on header write failure.
func Test_writeDummyFile_WriteHeader_Error(t *testing.T) {
	tw := tar.NewWriter(errorWriter{})
	err := writeDummyFile(tw, "fail.txt", false)

	require.Error(t, err)
	require.Contains(t, err.Error(), "header")
}

// Expecation: The channels should contain the correct ordered paths and no errors.
func Test_Program_tarPathStream_Sorted_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	tarData := createTar([]string{"z.txt", "b/", "b/c.txt"})
	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", tarData, 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.tarPathStream(t.Context(), "/archive.tar.gz", true, nil)

	got := []string{}
	for p := range paths {
		got = append(got, p)
	}

	for err := range errs {
		require.NoError(t, err)
	}

	require.Equal(t, []string{"b/", "b/c.txt", "z.txt"}, got)
}

// Expecation: The channels should contain the correct ordered paths and no errors.
func Test_Program_tarPathStream_Unsorted_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	tarData := createTar([]string{"z.txt", "b/", "b/c.txt"})
	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", tarData, 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.tarPathStream(t.Context(), "/archive.tar.gz", false, nil)

	got := []string{}
	for p := range paths {
		got = append(got, p)
	}

	for err := range errs {
		require.NoError(t, err)
	}

	require.Equal(t, []string{"z.txt", "b/", "b/c.txt"}, got)
}

// Expecation: The channels should contain the correct error and no paths.
func Test_Program_tarPathStream_Open_Error(t *testing.T) {
	baseFs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(baseFs, "/archive.tar.gz", []byte("test"), 0o644))

	fs := errorFs{Fs: baseFs}

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.tarPathStream(t.Context(), "/archive.tar.gz", false, nil)

	for range paths {
		t.Fatal("should not emit paths")
	}

	select {
	case err := <-errs:
		require.Error(t, err)
		require.Contains(t, err.Error(), "simulated open failure")
	default:
		t.Fatal("expected error from tarPathStream")
	}
}

// Expecation: The channels should contain the correct error and no paths.
func Test_Program_tarPathStream_GzipDecode_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", []byte("not a gzip file"), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.tarPathStream(t.Context(), "/archive.tar.gz", false, nil)

	for range paths {
		t.Fatal("should not emit any paths")
	}

	select {
	case err := <-errs:
		require.Error(t, err)
		require.Contains(t, err.Error(), "gzip")
	default:
		t.Fatal("expected gzip error from tarPathStream")
	}
}

// Expecation: The channels should contain the correct error and no paths.
func Test_Program_tarPathStream_TarDecode_Error(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)

	_, err := gz.Write([]byte("not a valid tarball"))
	require.NoError(t, err)

	err = gz.Close()
	require.NoError(t, err)

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", buf.Bytes(), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.tarPathStream(t.Context(), "/archive.tar.gz", false, nil)

	for range paths {
		t.Fatal("should not emit any paths")
	}

	select {
	case err := <-errs:
		require.Error(t, err)
		require.Contains(t, err.Error(), "tar")
	default:
		t.Fatal("expected tar error from tarPathStream")
	}
}

// Expectation: The channels should contain the correct ordered paths and no errors.
func Test_extsortStrings_Success(t *testing.T) {
	in := make(chan string, 3)
	in <- "c"
	in <- "a"
	in <- "b"
	close(in)

	extErrs := make(chan error)
	close(extErrs)

	out, errs := extsortStrings(t.Context(), in, extErrs, &extSortConfigDefault)

	got := []string{}
	for p := range out {
		got = append(got, p)
	}

	for err := range errs {
		require.NoError(t, err)
	}

	require.Equal(t, []string{"a", "b", "c"}, got)
}

// Expectation: The channels should contain the correct error and no paths.
func Test_extsortStrings_External_Error(t *testing.T) {
	in := make(chan string)
	close(in)

	extErrs := make(chan error, 1)
	extErrs <- errors.New("simulated external error")
	close(extErrs)

	out, errs := extsortStrings(t.Context(), in, extErrs, &extSortConfigDefault)

	for range out {
		t.Fatal("should not receive any output")
	}

	select {
	case err := <-errs:
		require.Error(t, err)
		require.Contains(t, err.Error(), "simulated external error")
	default:
		t.Fatal("expected error from extsortStrings")
	}
}

// Expectation: A context cancellation should be respected and the sorting interrupted.
func Test_extsortStrings_CtxCancel_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	in := make(chan string, 1)
	in <- "a"
	close(in)

	extErrs := make(chan error)
	close(extErrs)

	cancel()
	out, errs := extsortStrings(ctx, in, extErrs, &extSortConfigDefault)

	for range out {
		t.Fatal("should not emit output")
	}

	for err := range errs {
		require.ErrorIs(t, err, context.Canceled)
	}
}
