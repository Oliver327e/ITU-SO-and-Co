package main

import (
	"strings"
	"testing"
)

func TestValidMessage(test *testing.T) {
	testCases := []struct {
		messageText string
		expected    bool
	}{
		{"Hello!", true},
		{strings.Repeat("a", 128), true},
		{strings.Repeat("a", 129), false},
		{"Hej 👋", true},
		{"", false},
		{"   ", false},
		{string([]byte{0xff}), false},
	}

	for _, testCase := range testCases {
		result := validMessage(testCase.messageText)

		if result != testCase.expected {
			test.Errorf("validMessage(%q) = %v, expected %v",
				testCase.messageText, result, testCase.expected)
		}
	}
}
