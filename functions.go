package main

func StringSliceFind(slice []string, target string) int {
	for i, el := range slice {
		if el == target {
			return i
		}
	}
	return -1
}

func StringSliceRemove(slice []string, index int) []string {
	if index < 0 {
		return slice
	}
	slice[index] = slice[len(slice) - 1]
	return slice[:len(slice) - 1]
}