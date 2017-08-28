package models

// IsNotFoundError returns whether an error represents a "not found" error.
func IsNotFoundError(err error) bool {
	switch err.(type) {
	case ModelNotFoundError:
		return true
	}
	return false
}

// ModelNotFoundError represents when an instance is not found.
type ModelNotFoundError struct {
	modelName string
}

func (e ModelNotFoundError) Error() string {
	return e.modelName + " not found"
}
