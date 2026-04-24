package probe

import "io"

var (
	_ ResultWriter = (*LogWriter)(nil)
	_ ResultWriter = (*JSONFileWriter)(nil)
	_ io.Closer    = (*JSONFileWriter)(nil)
)
