//go:build !windows

package services

import "fmt"

func winClipWrite(_ string) error  { return fmt.Errorf("not windows") }
func winClipRead() (string, bool)  { return "", false }
func winSendCtrlV() error          { return fmt.Errorf("not windows") }
