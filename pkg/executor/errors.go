package executor

const (
	ErrSyntaxIncorrect     = "The syntax of the command is incorrect.\n"
	ErrFileNotFound        = "The system cannot find the file specified.\n"
	ErrPathNotFound        = "The system cannot find the path specified.\n"
	ErrAccessDenied        = "Access is denied.\n"
	ErrFileNotFoundPattern = "Could Not Find %s\n"
	ErrDirNotEmpty         = "The directory is not empty.\n"
	ErrFileExists          = "A duplicate file name exists, or the file cannot be found\n"
	ErrCannotCopyOntoSelf  = "The file cannot be copied onto itself.\n"
	ErrFileAlreadyExists   = "A subdirectory or file %s already exists.\n"
)
