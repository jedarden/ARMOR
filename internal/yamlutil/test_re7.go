package yamlutil

import (
	"fmt"
	"regexp"
)

func main() {
	re7 := regexp.MustCompile(`(?:unmarshal|marshal|convert)\s+(?:[\w\s]*)?\s*into\s+((?:chan|chan<-|<-chan)\s+[\w\-*]+|interface\{\}|[\[\]\*\w{}]+(?:\.[\w\-*]+)*)`)
	
	tests := []string{
		"cannot unmarshal into string",
		"convert into string",
		"marshal into int",
		"unmarshal X into bool",
	}
	
	for _, test := range tests {
		matches := re7.FindStringSubmatch(test)
		if matches != nil {
			fmt.Printf("Match: %q -> Type: %q\n", test, matches[1])
		} else {
			fmt.Printf("No match: %q\n", test)
		}
	}
}
