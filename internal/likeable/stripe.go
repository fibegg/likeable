package likeable

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (s *Server) stripeConfig(r *http.Request) (map[string]string, error) {
	cfg, err := s.store.ConfigMap(r.Context())
	if err != nil {
		return nil, err
	}
	out := stripeConfigFromMap(cfg)
	if out["secret"] == "" {
		return nil, fmt.Errorf("Stripe secret key is not configured")
	}
	return out, nil
}

func stripeConfigFromMap(cfg map[string]string) map[string]string {
	out := map[string]string{
		"secret":              strings.TrimSpace(cfg["stripe_secret_key"]),
		"price_1_hour":        strings.TrimSpace(cfg["stripe_price_id_1_hour"]),
		"price_10_hours":      strings.TrimSpace(cfg["stripe_price_id_10_hours"]),
		"price_100_hours":     strings.TrimSpace(cfg["stripe_price_id_100_hours"]),
		"project_quota_price": strings.TrimSpace(cfg["stripe_project_quota_price_id"]),
		"webhook":             strings.TrimSpace(cfg["stripe_webhook_secret"]),
	}
	return out
}

func (s *Server) billingProducts(ctx context.Context) map[string]any {
	cfg, err := s.store.ConfigMap(ctx)
	if err != nil {
		return emptyBillingProducts()
	}
	return billingProductsFromConfig(stripeConfigFromMap(cfg))
}

func billingProductsFromConfig(cfg map[string]string) map[string]any {
	if strings.TrimSpace(cfg["secret"]) == "" {
		return emptyBillingProducts()
	}
	hourPacks := make([]int, 0, 3)
	if strings.TrimSpace(cfg["price_1_hour"]) != "" {
		hourPacks = append(hourPacks, 1)
	}
	if strings.TrimSpace(cfg["price_10_hours"]) != "" {
		hourPacks = append(hourPacks, 10)
	}
	if strings.TrimSpace(cfg["price_100_hours"]) != "" {
		hourPacks = append(hourPacks, 100)
	}
	return map[string]any{
		"hourPacks":    hourPacks,
		"projectQuota": strings.TrimSpace(cfg["project_quota_price"]) != "",
	}
}

func emptyBillingProducts() map[string]any {
	return map[string]any{
		"hourPacks":    []int{},
		"projectQuota": false,
	}
}

func (s *Server) handleBillingCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user := userFromContext(r.Context())
	cfg, err := s.stripeConfig(r)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	var body struct {
		Pack    int    `json:"pack"`
		Product string `json:"product"`
		Slots   int    `json:"slots"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	product := normalizeStripeProduct(body.Product)
	pack := 0
	slots := 0
	priceID := ""
	switch product {
	case "project_quota":
		slots, priceID, err = stripeProjectQuotaPrice(cfg, body.Slots)
	default:
		pack, priceID, err = stripeHourPackPrice(cfg, body.Pack)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("client_reference_id", user.ID)
	form.Set("customer_email", user.Email)
	form.Set("success_url", s.config.BaseURL+"/profile?billing=success")
	form.Set("cancel_url", s.config.BaseURL+"/profile?billing=cancel")
	form.Set("metadata[purchase_kind]", product)
	if product == "project_quota" {
		form.Set("metadata[project_slots]", strconv.Itoa(slots))
		form.Set("metadata[project_quota_days]", "30")
	} else {
		form.Set("metadata[pack_hours]", strconv.Itoa(pack))
	}
	form.Set("line_items[0][price]", priceID)
	form.Set("line_items[0][quantity]", "1")
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, "https://api.stripe.com/v1/checkout/sessions", strings.NewReader(form.Encode()))
	req.SetBasicAuth(cfg["secret"], "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.http.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		writeError(w, http.StatusBadGateway, "stripe checkout failed: "+resp.Status)
		return
	}
	var out struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.URL == "" {
		writeError(w, http.StatusBadGateway, "stripe checkout response missing url")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": out.URL})
}

func normalizeStripeProduct(product string) string {
	switch strings.ToLower(strings.TrimSpace(product)) {
	case "project", "projects", "project_quota", "project-quota":
		return "project_quota"
	default:
		return "hour_pack"
	}
}

func stripeHourPackPrice(cfg map[string]string, pack int) (int, string, error) {
	switch pack {
	case 1:
		if cfg["price_1_hour"] == "" {
			return 0, "", fmt.Errorf("Stripe price for 1 hour is not configured")
		}
		return 1, cfg["price_1_hour"], nil
	case 10:
		if cfg["price_10_hours"] == "" {
			return 0, "", fmt.Errorf("Stripe price for 10 hours is not configured")
		}
		return 10, cfg["price_10_hours"], nil
	case 100:
		if cfg["price_100_hours"] == "" {
			return 0, "", fmt.Errorf("Stripe price for 100 hours is not configured")
		}
		return 100, cfg["price_100_hours"], nil
	default:
		return 0, "", fmt.Errorf("unsupported hour pack")
	}
}

func stripeProjectQuotaPrice(cfg map[string]string, slots int) (int, string, error) {
	if slots <= 0 {
		slots = 1
	}
	if slots != 1 {
		return 0, "", fmt.Errorf("unsupported project quota pack")
	}
	if cfg["project_quota_price"] == "" {
		return 0, "", fmt.Errorf("Stripe price for project quota is not configured")
	}
	return slots, cfg["project_quota_price"], nil
}

func (s *Server) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	body := readAll(r.Body)
	rawCfg, _ := s.store.ConfigMap(r.Context())
	cfg := stripeConfigFromMap(rawCfg)
	secret := cfg["webhook"]
	if secret == "" {
		writeError(w, http.StatusServiceUnavailable, "Stripe webhook secret is not configured")
		return
	}
	if !validStripeSignature(body, r.Header.Get("Stripe-Signature"), secret) {
		writeError(w, http.StatusBadRequest, "invalid stripe signature")
		return
	}
	var event struct {
		Type string `json:"type"`
		Data struct {
			Object map[string]any `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	switch event.Type {
	case "checkout.session.completed":
		userID := fmt.Sprint(event.Data.Object["client_reference_id"])
		customerID := fmt.Sprint(event.Data.Object["customer"])
		subscriptionID := fmt.Sprint(event.Data.Object["subscription"])
		paymentStatus := strings.ToLower(strings.TrimSpace(fmt.Sprint(event.Data.Object["payment_status"])))
		status := strings.ToLower(strings.TrimSpace(fmt.Sprint(event.Data.Object["status"])))
		if paymentStatus != "" && paymentStatus != "paid" {
			writeJSON(w, http.StatusOK, map[string]bool{"received": true})
			return
		}
		if status != "" && status != "complete" {
			writeJSON(w, http.StatusOK, map[string]bool{"received": true})
			return
		}
		if userID != "" {
			sessionID := fmt.Sprint(event.Data.Object["id"])
			if sessionID == "" || sessionID == "<nil>" {
				sessionID = subscriptionID
			}
			amountTotal := stripeInt64(event.Data.Object["amount_total"])
			currency := fmt.Sprint(event.Data.Object["currency"])
			if sessionID != "" && sessionID != "<nil>" {
				_ = s.store.UpsertPayment(r.Context(), Payment{
					UserID:            userID,
					ProviderPaymentID: sessionID,
					AmountCents:       amountTotal,
					Currency:          currency,
					Status:            "paid",
				})
			}
			packHours := stripePackHours(event.Data.Object["metadata"])
			if packHours > 0 {
				expectedPriceID := expectedStripeHourPackPrice(cfg, packHours)
				if err := s.verifyStripeCheckoutPrice(r.Context(), cfg, sessionID, expectedPriceID); err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
			}
			if packHours > 0 && sessionID != "" && sessionID != "<nil>" {
				if granted, err := s.store.GrantHourCredits(r.Context(), userID, sessionID, packHours); err == nil && granted {
					s.notifyHourCreditsPurchased(r.Context(), userID, packHours)
				}
			}
			projectSlots := stripeProjectSlots(event.Data.Object["metadata"])
			if projectSlots > 0 {
				if err := s.verifyStripeCheckoutPrice(r.Context(), cfg, sessionID, cfg["project_quota_price"]); err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
			}
			if projectSlots > 0 && sessionID != "" && sessionID != "<nil>" {
				expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)
				if granted, err := s.store.GrantProjectQuota(r.Context(), userID, sessionID, projectSlots, expiresAt); err == nil && granted {
					s.notifyProjectQuotaPurchased(r.Context(), userID, projectSlots, expiresAt)
				}
			}
			if subscriptionID != "" && subscriptionID != "<nil>" {
				_ = s.store.UpsertSubscription(r.Context(), Subscription{
					UserID:               userID,
					Status:               "active",
					StripeCustomerID:     customerID,
					StripeSubscriptionID: subscriptionID,
					CurrentPeriodEnd:     time.Now().UTC().Add(31 * 24 * time.Hour),
				})
			}
		}
	case "customer.subscription.deleted":
		// Stripe sends customer/subscription ids here; a follow-up improvement can map these exactly.
	}
	writeJSON(w, http.StatusOK, map[string]bool{"received": true})
}

func stripeInt64(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	default:
		return 0
	}
}

func stripePackHours(metadata any) int {
	raw := ""
	if m, ok := metadata.(map[string]any); ok {
		raw = fmt.Sprint(m["pack_hours"])
	}
	if raw == "" || raw == "<nil>" {
		return 0
	}
	n, _ := strconv.Atoi(raw)
	switch n {
	case 1, 10, 100:
		return n
	default:
		return 0
	}
}

func stripeProjectSlots(metadata any) int {
	raw := ""
	if m, ok := metadata.(map[string]any); ok {
		raw = fmt.Sprint(m["project_slots"])
		if raw == "" || raw == "<nil>" {
			raw = fmt.Sprint(m["project_quota"])
		}
	}
	if raw == "" || raw == "<nil>" {
		return 0
	}
	n, _ := strconv.Atoi(raw)
	switch n {
	case 1:
		return n
	default:
		return 0
	}
}

func expectedStripeHourPackPrice(cfg map[string]string, pack int) string {
	switch pack {
	case 1:
		return cfg["price_1_hour"]
	case 10:
		return cfg["price_10_hours"]
	case 100:
		return cfg["price_100_hours"]
	default:
		return ""
	}
}

func (s *Server) verifyStripeCheckoutPrice(ctx context.Context, cfg map[string]string, sessionID, expectedPriceID string) error {
	if strings.TrimSpace(expectedPriceID) == "" {
		return errors.New("Stripe price is not configured")
	}
	if strings.TrimSpace(sessionID) == "" || sessionID == "<nil>" {
		return errors.New("Stripe session id is missing")
	}
	if strings.TrimSpace(cfg["secret"]) == "" {
		return errors.New("Stripe secret key is not configured")
	}
	endpoint := "https://api.stripe.com/v1/checkout/sessions/" + url.PathEscape(sessionID) + "/line_items?limit=100"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.SetBasicAuth(cfg["secret"], "")
	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("stripe line item verification failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("stripe line item verification failed: %s", resp.Status)
	}
	var out struct {
		Data []struct {
			Price struct {
				ID string `json:"id"`
			} `json:"price"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("stripe line item verification failed: %w", err)
	}
	for _, item := range out.Data {
		if item.Price.ID == expectedPriceID {
			return nil
		}
	}
	return fmt.Errorf("stripe checkout line item did not match expected price")
}

func validStripeSignature(body []byte, header, secret string) bool {
	parts := strings.Split(header, ",")
	var timestamp, signature string
	for _, part := range parts {
		keyValue := strings.SplitN(part, "=", 2)
		if len(keyValue) != 2 {
			continue
		}
		switch keyValue[0] {
		case "t":
			timestamp = keyValue[1]
		case "v1":
			signature = keyValue[1]
		}
	}
	if timestamp == "" || signature == "" {
		return false
	}
	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	eventTime := time.Unix(unix, 0)
	if time.Since(eventTime) > 5*time.Minute || time.Until(eventTime) > 5*time.Minute {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
