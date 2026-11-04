// Package imagecapture persists asynchronously encoded notification images.
package imagecapture

// Save encodes and stores an image, then invalidates readers only after the
// successful write. The caller chooses whether to run Save asynchronously.
// An empty encoding does not create an image or signal an update.
func Save(encode func() []byte, store func([]byte) error, notify func()) error {
	data := encode()
	if len(data) == 0 {
		return nil
	}
	if err := store(data); err != nil {
		return err
	}
	notify()
	return nil
}
