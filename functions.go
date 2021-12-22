package main

import "errors"

func StringSliceFind(slice []string, target string) int {
	for i, el := range slice {
		if el == target {
			return i
		}
	}
	return -1
}

func StringSliceContains(slice []string, target string) bool {
	return StringSliceFind(slice, target) != -1
}

func StringSliceRemove(slice []string, index int) []string {
	if index < 0 {
		return slice
	}

	slice[index] = slice[len(slice)-1]
	return slice[:len(slice)-1]
}

func StringSliceRemoveTarget(slice []string, target string) []string {
	return StringSliceRemove(slice, StringSliceFind(slice, target))
}

func MapGetKeyByValue(myMap *map[string]string, myValue string) (string, error) {
	for key, value := range *myMap {
		if value == myValue {
			return key, nil
		}
	}
	return "", errors.New("not found")
}
