package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	 "net"
	"golang.org/x/net/http2"
	"context"
)

type PBSClient struct {
	baseurl string
	certfingerprint string 
	apitoken string 
	secret string
	authid string

	datastore string
	
	client http.Client
	tlsConfig tls.Config
}

func (pbs* PBSClient) CreateDynamicIndex(name string) {
	req, err := http.NewRequest("POST", pbs.baseurl+"/dynamic_index", bytes.NewBuffer([]byte(fmt.Sprintf( "{\"archive-name\": \"%s\"}", name))))
	if err != nil {
		panic(err)
	}
	
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s",pbs.authid, pbs.secret))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")


	resp2, err := pbs.client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		panic(err)
		return
	}

	if resp2.StatusCode != http.StatusOK {
		resp1 , err := io.ReadAll(resp2.Body)
		fmt.Println("Error making request:", string(resp1), string(resp2.Proto) )
		panic(err)
		return
	}
	defer resp2.Body.Close()
}

func (pbs* PBSClient) CloseDynamicIndex() {
	jsonpayload := fmt.Sprintf( "{\"chunk-count\": 0}")
	req, err := http.NewRequest("POST", pbs.baseurl+"/dynamic_close", bytes.NewBuffer([]byte(jsonpayload)))
	if err != nil {
		panic(err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s",pbs.authid, pbs.secret))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")


	resp2, err := pbs.client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		panic(err)
		return
	}
	defer resp2.Body.Close()
}

func (pbs* PBSClient) Finish() {
	req, err := http.NewRequest("POST", pbs.baseurl+"/finish", nil)
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s",pbs.authid, pbs.secret))
	if err != nil {
		panic(err)
	}
	resp2, err := pbs.client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		if err != nil {
			panic(err)
		}
		return
	}
	defer resp2.Body.Close()
}

func (pbs* PBSClient) Connect() {

	pbs.tlsConfig = tls.Config{
		InsecureSkipVerify: true, // Set to true if you want to skip certificate verification entirely (not recommended for production)
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			// Extract the peer certificate
			if len(rawCerts) == 0 {
				return fmt.Errorf("no certificates presented by the peer")
			}
			peerCert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return fmt.Errorf("failed to parse certificate: %v", err)
			}

			// Calculate the SHA-256 fingerprint of the certificate
			expectedFingerprint := strings.ReplaceAll(pbs.certfingerprint, ":", "")
			calculatedFingerprint := sha256.Sum256(peerCert.Raw)

			// Compare the calculated fingerprint with the expected one
			if hex.EncodeToString(calculatedFingerprint[:]) != expectedFingerprint {
				return fmt.Errorf("certificate fingerprint does not match (%s,%s)", expectedFingerprint, hex.EncodeToString(calculatedFingerprint[:]))
			}

			// If the fingerprint matches, the certificate is considered valid
			return nil
		},
		//ServerName: "127.0.0.1",

	}

	pbs.client = http.Client {
		Transport: &http2.Transport{

		DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
			conn, err := tls.Dial(network, addr, &pbs.tlsConfig)
			if err != nil {
				return nil, err
			}
			q := &url.Values{}
			q.Add("backup-time", fmt.Sprintf("%d",time.Now().Unix()))
			q.Add("backup-type", "host")
			q.Add("store", pbs.datastore)
			hostname, _:= os.Hostname()
			q.Add("backup-id", hostname)
			q.Add("debug", "1")
			conn.Write([]byte("GET /api2/json/backup?"+q.Encode()+" HTTP/1.1\r\n"))
			conn.Write([]byte("Authorization: "+fmt.Sprintf("PBSAPIToken=%s:%s",pbs.authid, pbs.secret)+"\r\n"))
			conn.Write([]byte("Upgrade: proxmox-backup-protocol-v1\r\n"))
			conn.Write([]byte("Connection: proxmox-backup-protocol-v1\r\n\r\n"))
			fmt.Println("Reading response to upgrade...\n")
			buf := make([]byte,0)
			for !strings.HasSuffix(string(buf),"\r\n\r\n") && !strings.HasSuffix(string(buf),"\n\n"){
				//fmt.Println(buf)
				b2 := make([]byte, 1)
				nbytes, err := conn.Read(b2)
				if err != nil || nbytes == 0 {
					fmt.Println("Connessione chiusa inasp.")
					return nil, err
				}
				buf = append(buf, b2[:nbytes]...)
				
				//fmt.Println(string(b2))
			}
			fmt.Printf("Upgraderesp: %s\n",string(buf))
			return conn, nil
		},
	},
		
	}




	/*req, err = http.NewRequest("POST", pbs.baseurl+"/api2/json/finish", nil)
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s",pbs.authid, pbs.secret))

	resp2, err := pbs.client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return
	}
	defer resp2.Body.Close()*/

	
	fmt.Println("Successfully upgraded to HTTP/2.")
}
