//go:build !windows

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// watchWindowSize, admin'in yerel terminali yeniden boyutlandığında (SIGWINCH)
// yeni boyutu aktif SSH oturumuna iletir (session.WindowChange). Böylece uzak
// PTY, oturum başında sabitlenen boyutta takılı kalmaz; tmux/vim/less gibi
// tam-ekran uygulamalar doğru şekilde yeniden çizer.
//
// Not: DB'ye kaydedilen cols/rows oturum başındaki değerdir (replay tutarlılığı
// için); canlı yeniden boyutlandırma yalnızca etkileşimli oturuma uygulanır.
func watchWindowSize(session *ssh.Session) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)

	fd := int(os.Stdin.Fd())
	for range sigCh {
		w, h, err := term.GetSize(fd)
		if err != nil {
			continue
		}
		if err := session.WindowChange(h, w); err != nil {
			log.Printf("[WARN] Terminal yeniden boyutlandırma iletilemedi: %v", err)
		}
	}
}
