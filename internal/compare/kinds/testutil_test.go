package kinds

import "os"

func readSQL(path string) (string, error) {
	b, err := os.ReadFile(path) //nolint:gosec // test fixture path
	if err != nil {
		return "", err
	}
	return string(b), nil
}
