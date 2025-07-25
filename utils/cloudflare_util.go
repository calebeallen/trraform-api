package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"trraformapi/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func ValidateTurnstileToken(httpCli *http.Client, ctx context.Context, token string) error {

	formData := url.Values{}
	formData.Set("secret", config.ENV.CF_TURNSTILE_SECRET_KEY)
	formData.Set("response", token)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(formData.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := httpCli.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("turnstile verify http %d", res.StatusCode)
	}

	var resp struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("turnstile verify failed: %v", resp.ErrorCodes)
	}

	return nil

}

func HasObjectR2(r2Cli *s3.Client, ctx context.Context, bucket string, key string) (bool, error) {

	_, err := r2Cli.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})

	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil

}

func PutObjectR2(r2Cli *s3.Client, ctx context.Context, bucket string, key string, body io.Reader, contentType string, metadata map[string]string) error {

	_, err := r2Cli.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		Body:        body,
		ContentType: aws.String(contentType),
		Metadata:    metadata,
	})
	if err != nil {
		return err
	}

	return nil

}

func UpdateMetadataR2(r2Cli *s3.Client, ctx context.Context, bucket string, key string, metadata map[string]string) error {

	_, err := r2Cli.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:            aws.String(bucket),
		CopySource:        aws.String(bucket + "/" + key),
		Key:               aws.String(key),
		Metadata:          metadata,
		MetadataDirective: types.MetadataDirectiveReplace, // IMPORTANT
	})
	if err != nil {
		return err
	}

	return nil

}

func GetObjectR2(r2Cli *s3.Client, ctx context.Context, bucket string, key string) ([]byte, map[string]string, error) {

	result, err := r2Cli.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, nil, err
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, nil, err
	}

	return data, result.Metadata, nil

}

func PurgeCacheCDN(httpCli *http.Client, ctx context.Context, urls []string) error {

	type Headers struct {
		Origin string `json:"Origin"`
	}
	type File struct {
		Url     string  `json:"url"`
		Headers Headers `json:"headers"`
	}
	type Payload struct {
		Files []File `json:"files"`
	}

	payload := Payload{Files: make([]File, len(urls))}
	for i, url := range urls {
		payload.Files[i] = File{
			Url: url,
			Headers: Headers{
				Origin: config.GEN.ORIGIN,
			},
		}
	}
	payloadBytes, err := json.Marshal(&payload)
	if err != nil {
		return err
	}

	apiUrl := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/purge_cache", config.ENV.CF_ZONE_ID)
	req, err := http.NewRequestWithContext(ctx, "POST", apiUrl, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.ENV.CF_API_TOKEN))
	req.Header.Set("Content-Type", "application/json")

	res, err := httpCli.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var purgeRes struct {
		Success bool     `json:"success"`
		Errors  []string `json:"errors"`
	}
	if err := json.NewDecoder(res.Body).Decode(&purgeRes); err != nil {
		return err
	}
	if !purgeRes.Success {
		return fmt.Errorf("purge failed: %v", purgeRes.Errors)
	}

	return nil

}
