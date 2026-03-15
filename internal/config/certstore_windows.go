package config

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"io"
	"math/big"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modNCrypt  = syscall.NewLazyDLL("ncrypt.dll")
	modCrypt32 = syscall.NewLazyDLL("crypt32.dll")

	procNCryptSignHash                    = modNCrypt.NewProc("NCryptSignHash")
	procNCryptFreeObject                  = modNCrypt.NewProc("NCryptFreeObject")
	procCryptAcquireCertificatePrivateKey = modCrypt32.NewProc("CryptAcquireCertificatePrivateKey")
)

const (
	certFindSubjectStr            = 0x00080007 // CERT_FIND_SUBJECT_STR_W
	x509AsnEncoding               = 0x00000001
	pkcs7AsnEncoding              = 0x00010000
	cryptAcquireOnlyNCryptKeyFlag = 0x00010000
	bcryptPadPKCS1                = 0x00000002
	bcryptPadPSS                  = 0x00000008
)

type bcryptPKCS1PaddingInfo struct {
	pszAlgId *uint16
}

type bcryptPSSPaddingInfo struct {
	pszAlgId *uint16
	cbSalt   uint32
}

func init() {
	loadClientCert = loadClientCertWindows
}

// loadClientCertWindows loads a client TLS certificate from the Windows
// certificate store. subject is matched against the certificate's subject
// string. storeName defaults to "My" (Personal).
func loadClientCertWindows(subject, storeName string) (*tls.Certificate, func(), error) {
	if storeName == "" {
		storeName = "My"
	}

	storeNamePtr, err := syscall.UTF16PtrFromString(storeName)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid store name: %w", err)
	}

	store, err := windows.CertOpenSystemStore(0, storeNamePtr)
	if err != nil {
		return nil, nil, fmt.Errorf("opening cert store %q: %w", storeName, err)
	}
	defer windows.CertCloseStore(store, 0)

	subjectPtr, err := syscall.UTF16PtrFromString(subject)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid subject: %w", err)
	}

	ctx, err := windows.CertFindCertificateInStore(
		store,
		x509AsnEncoding|pkcs7AsnEncoding,
		0,
		certFindSubjectStr,
		unsafe.Pointer(subjectPtr),
		nil,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("certificate with subject %q not found in store %q: %w", subject, storeName, err)
	}

	// Copy the raw cert bytes before freeing the context.
	rawCert := make([]byte, ctx.Length)
	copy(rawCert, unsafe.Slice(ctx.EncodedCert, ctx.Length))

	x509Cert, err := x509.ParseCertificate(rawCert)
	if err != nil {
		windows.CertFreeCertificateContext(ctx)
		return nil, nil, fmt.Errorf("parsing certificate: %w", err)
	}

	// Acquire the NCrypt private key handle.
	var keyHandle uintptr
	var keySpec uint32
	var mustFree int32
	r, _, callErr := procCryptAcquireCertificatePrivateKey.Call(
		uintptr(unsafe.Pointer(ctx)),
		cryptAcquireOnlyNCryptKeyFlag,
		0,
		uintptr(unsafe.Pointer(&keyHandle)),
		uintptr(unsafe.Pointer(&keySpec)),
		uintptr(unsafe.Pointer(&mustFree)),
	)
	windows.CertFreeCertificateContext(ctx)
	if r == 0 {
		return nil, nil, fmt.Errorf("acquiring private key for %q: %w", subject, callErr)
	}

	signer := &ncryptSigner{
		keyHandle: keyHandle,
		pub:       x509Cert.PublicKey,
	}

	cleanup := func() {
		procNCryptFreeObject.Call(keyHandle)
	}

	tlsCert := &tls.Certificate{
		Certificate: [][]byte{rawCert},
		PrivateKey:  signer,
		Leaf:        x509Cert,
	}

	return tlsCert, cleanup, nil
}

// ncryptSigner implements crypto.Signer using an NCrypt key handle.
type ncryptSigner struct {
	keyHandle uintptr
	pub       crypto.PublicKey
}

func (s *ncryptSigner) Public() crypto.PublicKey {
	return s.pub
}

func (s *ncryptSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	switch pub := s.pub.(type) {
	case *rsa.PublicKey:
		if pssOpts, ok := opts.(*rsa.PSSOptions); ok {
			return s.signRSAPSS(digest, pssOpts)
		}
		return s.signRSAPKCS1(digest, opts.HashFunc())
	case *ecdsa.PublicKey:
		return s.signECDSA(digest, pub)
	default:
		return nil, fmt.Errorf("unsupported key type: %T", pub)
	}
}

func (s *ncryptSigner) signRSAPKCS1(digest []byte, hash crypto.Hash) ([]byte, error) {
	algName, err := hashAlgName(hash)
	if err != nil {
		return nil, err
	}
	padding := bcryptPKCS1PaddingInfo{pszAlgId: algName}
	return s.ncryptSign(unsafe.Pointer(&padding), digest, bcryptPadPKCS1)
}

func (s *ncryptSigner) signRSAPSS(digest []byte, opts *rsa.PSSOptions) ([]byte, error) {
	hash := opts.HashFunc()
	algName, err := hashAlgName(hash)
	if err != nil {
		return nil, err
	}
	saltLen := opts.SaltLength
	if saltLen == rsa.PSSSaltLengthAuto || saltLen == rsa.PSSSaltLengthEqualsHash {
		saltLen = hash.Size()
	}
	padding := bcryptPSSPaddingInfo{
		pszAlgId: algName,
		cbSalt:   uint32(saltLen),
	}
	return s.ncryptSign(unsafe.Pointer(&padding), digest, bcryptPadPSS)
}

func (s *ncryptSigner) signECDSA(digest []byte, pub *ecdsa.PublicKey) ([]byte, error) {
	raw, err := s.ncryptSign(nil, digest, 0)
	if err != nil {
		return nil, err
	}
	// NCrypt returns raw r||s. Convert to ASN.1 DER for Go's TLS.
	halfLen := len(raw) / 2
	type ecdsaSig struct{ R, S *big.Int }
	return asn1.Marshal(ecdsaSig{
		R: new(big.Int).SetBytes(raw[:halfLen]),
		S: new(big.Int).SetBytes(raw[halfLen:]),
	})
}

func (s *ncryptSigner) ncryptSign(paddingInfo unsafe.Pointer, digest []byte, flags uint32) ([]byte, error) {
	paddingPtr := uintptr(0)
	if paddingInfo != nil {
		paddingPtr = uintptr(paddingInfo)
	}

	// First call: determine output size.
	var cbResult uint32
	status, _, _ := procNCryptSignHash.Call(
		s.keyHandle, paddingPtr,
		uintptr(unsafe.Pointer(&digest[0])), uintptr(len(digest)),
		0, 0,
		uintptr(unsafe.Pointer(&cbResult)),
		uintptr(flags),
	)
	if status != 0 {
		return nil, fmt.Errorf("NCryptSignHash (size query) failed: 0x%x", status)
	}

	// Second call: sign.
	sig := make([]byte, cbResult)
	status, _, _ = procNCryptSignHash.Call(
		s.keyHandle, paddingPtr,
		uintptr(unsafe.Pointer(&digest[0])), uintptr(len(digest)),
		uintptr(unsafe.Pointer(&sig[0])), uintptr(len(sig)),
		uintptr(unsafe.Pointer(&cbResult)),
		uintptr(flags),
	)
	if status != 0 {
		return nil, fmt.Errorf("NCryptSignHash failed: 0x%x", status)
	}

	return sig[:cbResult], nil
}

func hashAlgName(h crypto.Hash) (*uint16, error) {
	var name string
	switch h {
	case crypto.SHA1:
		name = "SHA1"
	case crypto.SHA256:
		name = "SHA256"
	case crypto.SHA384:
		name = "SHA384"
	case crypto.SHA512:
		name = "SHA512"
	default:
		return nil, fmt.Errorf("unsupported hash algorithm: %v", h)
	}
	p, _ := syscall.UTF16PtrFromString(name)
	return p, nil
}
