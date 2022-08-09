// +build !go1.10

package grpcrunner

func indent() string {
	// In Go 1.9 and older, we need to add indentation
	// after newlines in the flag doc strings.
	return "    \t"
}
