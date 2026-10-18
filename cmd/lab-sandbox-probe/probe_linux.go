package main

import "syscall"

// keyring consulta o chaveiro de sessão do kernel com keyctl. A chamada
// não exige privilégio nenhum, e o perfil seccomp padrão do Docker a
// bloqueia: o chaveiro não é isolado por contêiner.
func keyring() error {
	const getKeyringID = 0
	session := int64(-3) // KEY_SPEC_SESSION_KEYRING
	//nolint:gosec // G115: -3 vira o padrão de bits que o kernel espera
	_, _, errno := syscall.Syscall(syscall.SYS_KEYCTL, getKeyringID,
		uintptr(session), 0)
	if errno != 0 {
		return errno
	}
	return nil
}

// setuidRoot tenta trocar o usuário do processo por root.
func setuidRoot() error { return syscall.Setuid(0) }
