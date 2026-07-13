//go:build windows

package main

import "golang.org/x/crypto/ssh"

// watchWindowSize, Windows'ta işlemsizdir: SIGWINCH yoktur ve ConPTY yeniden
// boyutlandırma tespiti farklı bir mekanizma gerektirir. Oturum başındaki PTY
// boyutu kullanılmaya devam eder.
func watchWindowSize(session *ssh.Session) {}
