package rig

import "golang.org/x/crypto/ssh"

func (c *SSH) Client() *ssh.Client {
	return c.client
}
