/* 
   Copyright 2011 John Asmuth

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package main

import (
	"testing"
	"fmt"
)

type GRTest struct {
	start, finish, wd string
	truth string
}


func TestGetRelative(t *testing.T) {
	grTests := []GRTest{
	
	}
	grTestsWindows := []GRTest{
		{"C:/c/go/gc", "C:\\c\\go\\go-gb\\example", "wd_does_not_matter_here", "../go-gb/example"},
	}

	for _, grt := range grTests {
		result := GetRelative(grt.start, grt.finish, grt.wd)
		if result != grt.truth {
			t.Error(fmt.Sprintf("GetRelative(\"%s\", \"%s\", \"%s\") -> \"%s\", was expecting \"%s\"", grt.start, grt.finish, grt.wd, result, grt.truth))
		}
	}

	TestWindows = true
	for _, grt := range grTestsWindows {
		result := GetRelative(grt.start, grt.finish, grt.wd)
		if result != grt.truth {
			t.Error(fmt.Sprintf("GetRelative(\"%s\", \"%s\", \"%s\") -> \"%s\", was expecting \"%s\"", grt.start, grt.finish, grt.wd, result, grt.truth))
		}
	}
	TestWindows = false
}