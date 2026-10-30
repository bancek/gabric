package gabric

import "io"

// copyAndCloseWriter copies src into dst and closes dst. The copy error takes
// precedence over the close error.
func copyAndCloseWriter(dst io.WriteCloser, src io.Reader) error {
	_, err := io.Copy(dst, src)
	if closeErr := dst.Close(); err == nil {
		err = closeErr
	}
	return err
}

// copyAndCloseReader copies src into dst and closes src. The copy error takes
// precedence over the close error.
func copyAndCloseReader(src io.ReadCloser, dst io.Writer) error {
	_, err := io.Copy(dst, src)
	if closeErr := src.Close(); err == nil {
		err = closeErr
	}
	return err
}
