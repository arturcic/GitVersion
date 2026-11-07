package main

import (
	"errors"
	"os"
)

type fileChange struct {
	path, content string
	original      []byte
	info          os.FileInfo
}

func applyFileChanges(changes []fileChange) error {
	for i, item := range changes {
		var err error
		if item.info != nil {
			err = writeFileAtomically(item.path, []byte(item.content), item.info, true)
		} else {
			err = createAssemblyInfo(item.path, item.content)
		}
		if err != nil {
			for j := i - 1; j >= 0; j-- {
				previous := changes[j]
				if previous.info == nil {
					err = errors.Join(err, os.Remove(previous.path))
				} else {
					err = errors.Join(err, writeFileAtomically(previous.path, previous.original, previous.info, true))
				}
			}
			return err
		}
	}
	return nil
}
