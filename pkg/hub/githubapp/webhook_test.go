// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package githubapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestVerifyWebhookSignature(t *testing.T) {
	secret := "whsec_test_secret_123"
	payload := []byte(`{"action":"created","installation":{"id":12345}}`)

	// Compute a valid signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	validSig := fmt.Sprintf("sha256=%x", mac.Sum(nil))

	t.Run("valid signature", func(t *testing.T) {
		if !VerifyWebhookSignature(payload, validSig, secret) {
			t.Error("expected valid signature to pass verification")
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		if VerifyWebhookSignature(payload, "sha256=deadbeef", secret) {
			t.Error("expected invalid signature to fail verification")
		}
	})

	t.Run("wrong prefix", func(t *testing.T) {
		if VerifyWebhookSignature(payload, "sha1="+validSig[7:], secret) {
			t.Error("expected wrong prefix to fail verification")
		}
	})

	t.Run("empty secret", func(t *testing.T) {
		if VerifyWebhookSignature(payload, validSig, "") {
			t.Error("expected empty secret to fail verification")
		}
	})

	t.Run("empty signature", func(t *testing.T) {
		if VerifyWebhookSignature(payload, "", secret) {
			t.Error("expected empty signature to fail verification")
		}
	})

	t.Run("modified payload", func(t *testing.T) {
		modified := []byte(`{"action":"deleted","installation":{"id":12345}}`)
		if VerifyWebhookSignature(modified, validSig, secret) {
			t.Error("expected modified payload to fail verification")
		}
	})
}
