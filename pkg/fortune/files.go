package fortune

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/vromero/gofortune/pkg"
)

// Represents a file/directory path with the possibilities established by the requester
// that the file should be selected in a random pick.
// Path should point only to directories containing fortune files or to fortune files
// with a ".dat" file present for it.
type ProbabilityPath struct {
	Path       string
	Percentage float32
}

type FileSystemNodeDescriptor struct {
	Percent                  float32
	UndefinedChildrenPercent float32 // Total percentage non user-defined for this node
	UndefinedNumEntries      uint64
	NumEntries               uint64 // Total number of fortunes in all files
	NumFiles                 int    // Total number of files
	Path                     string
	IndexPath                string
	Table                    pkg.DataTable
	isUtf8                   bool
	Children                 []FileSystemNodeDescriptor
	Parent                   *FileSystemNodeDescriptor
}

// LoadPaths Loads the paths described in the paths arguments returning
// a FileSystemNodeDescriptor that includes extra information as the Table and
// all the children (if a directory is passed).
// LoadPaths can filter fortune files by the shortest or longest dictum it has.
// This is useful to prevent infinite loops.
func LoadPaths(paths []ProbabilityPath, shorterThan uint32, longerThan uint32) (FileSystemNodeDescriptor, error) {
	rootFsDescriptor := FileSystemNodeDescriptor{
		Percent: 100,
	}

	for i := range paths {
		err := loadPath(paths[i], &rootFsDescriptor, shorterThan, longerThan)
		if err != nil {
			return rootFsDescriptor, err
		}
	}
	return rootFsDescriptor, nil
}

func loadPath(path ProbabilityPath, parent *FileSystemNodeDescriptor, shorterThan uint32, longerThan uint32) (err error) {
	fsDescriptor := FileSystemNodeDescriptor{
		Path:    path.Path,
		Percent: path.Percentage,
		Parent:  parent,
	}

	stat, err := os.Stat(path.Path)
	if err != nil {
		return err
	}
	if stat.IsDir() {
		err = loadDirPath(&fsDescriptor, parent, shorterThan, longerThan)
		if err != nil {
			return err
		}
	} else {
		err = loadFilePath(&fsDescriptor, parent, shorterThan, longerThan)
		if err != nil {
			return err
		}
	}
	return nil
}

func loadDirPath(fsDescriptor *FileSystemNodeDescriptor, parent *FileSystemNodeDescriptor, shorterThan uint32, longerThan uint32) error {
	fsNodes, err := ioutil.ReadDir(fsDescriptor.Path)
	if err != nil {
		return err
	}

	for i := range fsNodes {
		// Sub-directories are ignored for compatibility with the original fortune and because
		// all cookies are typically stored in sub-directories of /usr/share/games/fortune
		if !fsNodes[i].IsDir() {
			childFsDescriptor := FileSystemNodeDescriptor{
				Path:   filepath.Join(fsDescriptor.Path, fsNodes[i].Name()),
				Parent: fsDescriptor,
			}

			// Files that are invalid will be ignored
			_ = loadFilePath(&childFsDescriptor, fsDescriptor, shorterThan, longerThan)
		}
	}

	fsDescriptor.Parent = parent
	parent.Children = append(parent.Children, *fsDescriptor)
	return nil
}

func loadFilePath(fsDescriptor *FileSystemNodeDescriptor, parent *FileSystemNodeDescriptor, shorterThan uint32, longerThan uint32) error {
	if !isFortuneFile(fsDescriptor.Path) {
		return errors.New("file is not a valid fortune file")
	}

	indexPath := fsDescriptor.Path + ".dat"
	if !isFortuneIndexFile(indexPath) {
		return errors.New("file is not a valid fortune index file")
	}
	fsDescriptor.IndexPath = indexPath

	table, err := pkg.LoadDataTableFromPath(fsDescriptor.IndexPath)
	if err != nil {
		return err
	}

	if table.LongestLength < longerThan || table.ShortestLength > shorterThan {
		return errors.New("file do not honor the length filter")
	}

	fsDescriptor.Table = table
	fsDescriptor.Parent = parent

	populateFileAmounts(fsDescriptor, table)
	parent.Children = append(parent.Children, *fsDescriptor)
	return nil
}

func populateFileAmounts(fsDescriptor *FileSystemNodeDescriptor, table pkg.DataTable) {
	current := fsDescriptor
	for {
		current.NumEntries += uint64(table.NumberOfStrings)
		current.NumFiles++

		current = current.Parent
		if current == nil {
			break
		}
	}
}

// Assert if a file is a fortune index file
func isFortuneFile(path string) bool {
	return pkg.FileExists(path)
}

// Assert if a file is a fortune index file
func isFortuneIndexFile(path string) bool {

	// If the file has not an associated fortune index file it should be ignored
	if !pkg.FileExists(path) {
		return false
	}

	// If the associated fortune index is has not the correct version
	version, err := pkg.LoadDataTableVersionFromPath(path)
	if err != nil || version.Version != pkg.DefaultVersion {
		return false
	}

	return true
}
