package main

func StringSliceFind(slice []string, target string) int {
	for i, el := range slice {
		if el == target {
			return i
		}
	}
	return -1
}