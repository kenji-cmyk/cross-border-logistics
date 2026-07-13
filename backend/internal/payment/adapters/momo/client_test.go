package momo

import (
	"fmt"
	"testing"
	"time"
)

func TestVerifyIPN(t *testing.T) {
	c, _ := New(Config{PartnerCode: "MOMO", AccessKey: "access", SecretKey: "secret", BaseURL: "https://example.com", IPNURL: "https://example.com/ipn", RedirectURL: "https://example.com/return", Timeout: time.Second})
	raw := "accessKey=access&amount=10000&extraData=&message=Successful.&orderId=payment-1&orderInfo=CrossBorder&orderType=momo_wallet&partnerCode=MOMO&payType=qr&requestId=request-1&responseTime=1700000000000&resultCode=0&transId=123"
	sig := sign("secret", raw)
	body := []byte(fmt.Sprintf(`{"partnerCode":"MOMO","orderId":"payment-1","requestId":"request-1","amount":10000,"orderInfo":"CrossBorder","orderType":"momo_wallet","transId":123,"resultCode":0,"message":"Successful.","payType":"qr","responseTime":1700000000000,"extraData":"","signature":"%s"}`, sig))
	p, err := c.VerifyIPN(body)
	if err != nil || p.TransID != 123 {
		t.Fatalf("p=%+v err=%v", p, err)
	}
	bad := append([]byte(nil), body...)
	bad[len(bad)-3] = '0'
	if _, err = c.VerifyIPN(bad); err == nil {
		t.Fatal("tampered signature accepted")
	}
}
