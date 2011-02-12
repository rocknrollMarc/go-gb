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

//target:gb
package main

import (
	"strings"
	"os"
	"fmt"
	"path"
)

// command line flags
var Install, //-i
	Clean,            //-c
	Nuke,             //-N
	Scan,             //-sS
	ScanList,         //-S
	Test,             //-t
	Exclusive,        //-e
	BuildGOROOT,      //-R
	GoInstall,        //-gG
	GoInstallUpdate,  //-G
	Concurrent,       //-p
	Verbose,          //-v
	GenMake,          //-M
	Build,            //-b
	Force,            //-f
	Makefiles,        //-m
	GoFMT,            //-F
	DoPkgs,           //-P
	DoCmds,           //-C
	Distribution bool //-D

var IncludeDir string
var GCArgs []string
var GLArgs []string
var PackagesBuilt int
var PackagesInstalled int
var BrokenPackages int
var ListedTargets int
var ListedDirs map[string]bool
var ListedPkgs []*Package

var BrokenMsg []string
var ReturnFailCode bool

var RunningInGOROOT bool

var buildBlock chan bool
var Packages = make(map[string]*Package)


func ScanDirectory(base, dir string) (err2 os.Error) {
	_, basedir := path.Split(dir)
	if basedir == "_obj" ||
		basedir == "_test" ||
		basedir == "_dist_" ||
		basedir == "bin" ||
		(basedir != "." && strings.HasPrefix(basedir, ".")) {
		return
	}

	var err os.Error

	var pkg *Package
	pkg, err = NewPackage(base, dir)
	if err == nil {
		Packages["\""+pkg.Target+"\""] = pkg
		base = pkg.Base
	} else {
		if tbase, terr := DirTargetGB(dir); terr == nil {
			base = tbase
		}
	}

	if pkg.Target == "." {
		err = os.NewError("Package has no name specified. Either create 'target.gb' or run gb from above.")
	}

	subdirs := GetSubDirs(dir)
	for _, subdir := range subdirs {
		if subdir != "src" {
			ScanDirectory(path.Join(base, subdir), path.Join(dir, subdir))
		}
	}

	return
}

func IsListed(name string) bool {
	if ListedTargets == 0 {
		return true
	}
	if Exclusive {
		return ListedDirs[name]
	}

	for lt := range ListedDirs {
		if HasPathPrefix(name, lt) {
			return true
		}
	}
	return false
}

func MakeDist(ch chan string) (err os.Error) {
	fmt.Printf("Removing _dist_\n")
	if err = os.RemoveAll("_dist_"); err != nil {
		return
	}

	if err = os.MkdirAll("_dist_", 0755); err != nil {
		return
	}

	fmt.Printf("Copying distribution files to _dist_\n")
	for file := range ch {
		if _, err = os.Stat(file); err != nil {
			fmt.Printf("Couldn't find '%s' for copy to _dist_.\n", file)
			return
		}
		nfile := path.Join("_dist_", file)
		npdir, _ := path.Split(nfile)
		if err = os.MkdirAll(npdir, 0755); err != nil {
			fmt.Printf("Couldn't create directory '%s'.\n", npdir)
			return
		}
		Copy(".", file, nfile)
	}

	return
}

func TryScan() {
	if Scan {
		for _, pkg := range Packages {
			if pkg.IsInGOROOT && !RunningInGOROOT {
				continue
			}
			if IsListed(pkg.Dir) {
				pkg.PrintScan()
			}
		}
		return
	}
}

func TryGoFMT() (err os.Error) {
	if GoFMT {
		for _, pkg := range ListedPkgs {
			err = pkg.GoFMT()
			if err != nil {
				return
			}
		}
	}
	return
}

func TryGenMake() (err os.Error) {
	if GenMake {
		_, ferr := os.Stat("build")

		genBuild := true

		if ferr == nil {
			if !Force {
				fmt.Printf("'build' exists; overwrite? (y/n) ")
				var answer string
				fmt.Scanf("%s", &answer)
				genBuild = answer == "y" || answer == "Y"
			}
			os.Remove("build")
		}

		if genBuild {
			fmt.Printf("(in .) generating build script\n")
			var buildFile *os.File
			buildFile, err = os.Open("build", os.O_CREATE|os.O_RDWR, 0755)
			_, err = fmt.Fprintf(buildFile, "# Build script generated by gb: http://go-gb.googlecode.com\n")
			_, err = fmt.Fprintf(buildFile, "# gb provides configuration-free building and distributing\n")
			_, err = fmt.Fprintf(buildFile, "\n")
			_, err = fmt.Fprintf(buildFile, "echo \"Build script generated by gb: http://go-gb.googlecode.com\" \\\n")
			for _, pkg := range ListedPkgs {
				pkg.AddToBuild(buildFile)
			}
			_, err = fmt.Fprintf(buildFile, "\n# The makefiles above are invoked in topological dependence order\n")
			buildFile.Close()
		}
		for _, pkg := range ListedPkgs {
			err = pkg.GenerateMakefile()
			if err != nil {
				return
			}
		}
	}

	return
}

func TryDistribution() (err os.Error) {
	if Distribution {
		ch := make(chan string)
		go func() {
			tryFile := func(name string) bool {
				_, ferr := os.Stat(name)
				if ferr == nil {
					ch <- name
					return true
				}
				return false
			}
			tryFile("build")
			tryFile("README")

			LineChan("dist.gb", ch)

			for _, pkg := range ListedPkgs {
				err = pkg.CollectDistributionFiles(ch)
				if err != nil {
					return
				}
			}
			close(ch)
		}()
		err = MakeDist(ch)
		if err != nil {
			return
		}
	}
	return
}

func TryClean() {
	if Clean && ListedTargets == 0 {
		fmt.Println("Removing " + GetBuildDirPkg())
		os.RemoveAll(GetBuildDirPkg())
		fmt.Println("Removing " + GetBuildDirCmd())
		os.RemoveAll(GetBuildDirCmd())
	}

	if Clean {
		for _, pkg := range ListedPkgs {
			pkg.Clean()
		}
	}
}

func TryBuild() {

	if Build {
		if Concurrent {
			for _, pkg := range ListedPkgs {
				pkg.CheckStatus()
				go pkg.Build()
			}
		}
		for _, pkg := range ListedPkgs {
			pkg.CheckStatus()
			err := pkg.Build()
			if err != nil {

			}
		}
	}

	if len(BrokenMsg) != 0 {
		for _, msg := range BrokenMsg {
			fmt.Printf("%s\n", msg)
		}
	}
}

func TryTest() (err os.Error) {
	if Test {
		for _, pkg := range ListedPkgs {
			if len(pkg.TestSources) != 0 {
				err = pkg.Test()
				if err != nil {
					return
				}
			}
		}
	}
	return
}

func TryInstall() {
	if Install {
		brokenMsg := []string{}
		for _, pkg := range ListedPkgs {
			err := pkg.Install()
			if err != nil {
				brokenMsg = append(brokenMsg, fmt.Sprintf("(in %s) could not install \"%s\"", pkg.Dir, pkg.Target))
			}
		}

		if len(brokenMsg) != 0 {
			for _, msg := range brokenMsg {
				fmt.Printf("%s\n", msg)
			}
		}
	}
}

func RunGB() (err os.Error) {
	Build = Build || (!GenMake && !Clean && !GoFMT && !Scan) || (Makefiles && !Clean) || Install || Test

	DoPkgs, DoCmds = DoPkgs || (!DoPkgs && !DoCmds), DoCmds || (!DoPkgs && !DoCmds)

	ListedDirs = make(map[string]bool)

	args := os.Args[1:len(os.Args)]

	err = ScanDirectory(".", ".")
	if err != nil {
		return
	}
	if BuildGOROOT {
		fmt.Printf("Scanning %s...", path.Join("GOROOT", "src"))
		ScanDirectory("", path.Join(GOROOT, "src"))
		fmt.Printf("done\n")
	}

	for _, arg := range args {
		if arg[0] != '-' {
			ListedDirs[path.Clean(arg)] = true
		}
	}

	ListedPkgs = []*Package{}
	for _, pkg := range Packages {
		if !RunningInGOROOT && pkg.IsInGOROOT {
			continue
		}
		if IsListed(pkg.Dir) {
			ListedPkgs = append(ListedPkgs, pkg)
		}
	}

	for _, pkg := range Packages {
		pkg.Stat()
	}

	for _, pkg := range Packages {
		pkg.ResolveDeps()
	}

	for _, pkg := range Packages {
		pkg.CheckStatus()
	}

	TryScan()

	if err = TryGoFMT(); err != nil {
		return
	}

	if err = TryGenMake(); err != nil {
		return
	}

	if err = TryDistribution(); err != nil {
		return
	}

	TryClean()

	TryBuild()

	if err = TryTest(); err != nil {
		return
	}

	TryInstall()

	if !Clean {
		if PackagesBuilt > 1 {
			fmt.Printf("Built %d targets\n", PackagesBuilt)
		} else if PackagesBuilt == 1 {
			println("Built 1 target")
		}
		if PackagesInstalled > 1 {
			fmt.Printf("Installed %d targets\n", PackagesInstalled)
		} else if PackagesInstalled == 1 {
			println("Installed 1 target")
		}
		if Build && PackagesBuilt == 0 && PackagesInstalled == 0 && BrokenPackages == 0 {
			println("Up to date")
		}
		if BrokenPackages > 1 {
			fmt.Printf("%d broken targets\n", BrokenPackages)
		} else if BrokenPackages == 1 {
			println("1 broken target")
		}
	} else {
		if PackagesBuilt == 0 {
			println("No mess to clean")
		}
	}

	return
}

func CheckFlags() {
	for _, arg := range os.Args[1:len(os.Args)] {
		if len(arg) > 0 && arg[0] == '-' {
			for _, flag := range arg[1:len(arg)] {
				switch flag {
				case 'i':
					Install = true
				case 'c':
					Clean = true
				case 'N':
					Clean = true
					Nuke = true
				case 'b':
					Build = true
				case 's':
					Scan = true
				case 'S':
					Scan = true
					ScanList = true
				case 't':
					Test = true
				case 'e':
					Exclusive = true
				case 'v':
					Verbose = true
				case 'm':
					Makefiles = true
				case 'M':
					GenMake = true
				case 'f':
					Force = true
				case 'g':
					GoInstall = true
				case 'G':
					GoInstall = true
					GoInstallUpdate = true
				case 'p':
					Concurrent = true
				case 'F':
					GoFMT = true
				case 'P':
					DoPkgs = true
				case 'C':
					DoCmds = true
				case 'D':
					Distribution = true
				case 'R':
					BuildGOROOT = true
				default:
					Usage()
					return

				}
			}
		} else {
			ListedTargets++
		}
	}
}

func main() {
	/*
		r, _ := GetRelative("e:\\tmp\\go-etc\\mingw4~1\\go", ".")
		println(r)
	*/

	LoadEnvs()

	CheckFlags()

	err := FindExternals()
	if err != nil {
		return
	}

	GCArgs = []string{}
	GLArgs = []string{}

	if !Install {
		IncludeDir = GetBuildDirPkg()
		GCArgs = append(GCArgs, []string{"-I", IncludeDir}...)
		GLArgs = append(GLArgs, []string{"-L", IncludeDir}...)
	}

	err = RunGB()
	if err != nil {
		fmt.Printf("%v\n", err)
		ReturnFailCode = true
	}

	if len(BrokenMsg) > 0 {
		ReturnFailCode = true
	}

	if ReturnFailCode {
		os.Exit(1)
	}
}
