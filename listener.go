package libp2pquic

import (
	"net"

	ic "gx/ipfs/QmNiJiXwWE3kRhZrC5ej3kSjWHm337pYfhjLGSCDNKJP2s/go-libp2p-crypto"
	manet "gx/ipfs/QmQVUtnrNGtCRkCMpXgpApfzQjc8FDaDVxHqWH8cnZQeh5/go-multiaddr-net"
	ma "gx/ipfs/QmRKLtwMw131aK7ugC3G7ybpumMz78YrJe5dzneyindvG1/go-multiaddr"
	quic "gx/ipfs/QmU44KWVkSHno7sNDTeUcL4FBgxgoidkFuTUyTXWJPXXFJ/quic-go"
	tls "gx/ipfs/QmUT6HZJnZhpZPiCtrsBmRnpfbysEgBNJESVQe8wuuezX6/go-libp2p-tls"
	peer "gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	tpt "gx/ipfs/Qmb3qartY8DSgRaBA3Go4EEjY1ZbXhCcvmc4orsBKMjgRg/go-libp2p-transport"
)

var quicListenAddr = quic.ListenAddr

// A listener listens for QUIC connections.
type listener struct {
	quicListener quic.Listener
	transport    tpt.Transport

	privKey        ic.PrivKey
	localPeer      peer.ID
	localMultiaddr ma.Multiaddr
}

var _ tpt.Listener = &listener{}

func newListener(addr ma.Multiaddr, transport tpt.Transport, localPeer peer.ID, key ic.PrivKey, identity *tls.Identity) (tpt.Listener, error) {
	lnet, host, err := manet.DialArgs(addr)
	if err != nil {
		return nil, err
	}
	laddr, err := net.ResolveUDPAddr(lnet, host)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP(lnet, laddr)
	if err != nil {
		return nil, err
	}
	ln, err := quic.Listen(conn, identity.Config, quicConfig)
	if err != nil {
		return nil, err
	}
	localMultiaddr, err := toQuicMultiaddr(ln.Addr())
	if err != nil {
		return nil, err
	}
	return &listener{
		quicListener:   ln,
		transport:      transport,
		privKey:        key,
		localPeer:      localPeer,
		localMultiaddr: localMultiaddr,
	}, nil
}

// Accept accepts new connections.
func (l *listener) Accept() (tpt.Conn, error) {
	for {
		sess, err := l.quicListener.Accept()
		if err != nil {
			return nil, err
		}
		conn, err := l.setupConn(sess)
		if err != nil {
			sess.CloseWithError(0, err)
			continue
		}
		return conn, nil
	}
}

func (l *listener) setupConn(sess quic.Session) (tpt.Conn, error) {
	remotePubKey, err := tls.KeyFromChain(sess.ConnectionState().PeerCertificates)
	if err != nil {
		return nil, err
	}
	remotePeerID, err := peer.IDFromPublicKey(remotePubKey)
	if err != nil {
		return nil, err
	}
	remoteMultiaddr, err := toQuicMultiaddr(sess.RemoteAddr())
	if err != nil {
		return nil, err
	}
	return &conn{
		sess:            sess,
		transport:       l.transport,
		localPeer:       l.localPeer,
		localMultiaddr:  l.localMultiaddr,
		privKey:         l.privKey,
		remoteMultiaddr: remoteMultiaddr,
		remotePeerID:    remotePeerID,
		remotePubKey:    remotePubKey,
	}, nil
}

// Close closes the listener.
func (l *listener) Close() error {
	return l.quicListener.Close()
}

// Addr returns the address of this listener.
func (l *listener) Addr() net.Addr {
	return l.quicListener.Addr()
}

// Multiaddr returns the multiaddress of this listener.
func (l *listener) Multiaddr() ma.Multiaddr {
	return l.localMultiaddr
}
