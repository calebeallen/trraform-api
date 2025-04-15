package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/joho/godotenv"
)

var s3Client *s3.Client

func init() {

	/* for development */
	if os.Getenv("ENV") != "prod" {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
			return
		}
	}

	cred := credentials.NewStaticCredentialsProvider(os.Getenv("CF_R2_ACCESS_KEY"), os.Getenv("CF_R2_SECRET_KEY"), "")
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithCredentialsProvider(cred))
	if err != nil {
		return
	}

	s3Client = s3.New(s3.Options{
		Credentials:  cfg.Credentials,
		BaseEndpoint: aws.String(os.Getenv("CF_R2_API_ENDPOINT")),
		UsePathStyle: true,
		Region:       "auto",
	})

}

func HasObjectR2(ctx context.Context, bucket, key string) (bool, error) {

	if s3Client == nil {
		return false, fmt.Errorf("in HasObjectR2:\ncould not init s3Client")
	}

	_, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})

	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("in HasObjectR2: %w", err)
	}

	return true, nil

}

func PutObjectR2(ctx context.Context, bucket string, key string, body io.Reader, contentType string, metadata map[string]string) error {

	if s3Client == nil {
		return fmt.Errorf("in PutObjectR2:\ncould not init s3Client")
	}

	_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		Body:        body,
		ContentType: aws.String(contentType),
		Metadata:    metadata,
	})
	if err != nil {
		return fmt.Errorf("in PutObjectR2:\n%w", err)
	}

	return nil

}

func UpdateMetadataR2(ctx context.Context, bucket string, key string, metadata map[string]string) error {

	if s3Client == nil {
		return fmt.Errorf("in PutObjectR2:\ncould not init s3Client")
	}

	_, err := s3Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:            aws.String(bucket),
		CopySource:        aws.String(bucket + "/" + key),
		Key:               aws.String(key),
		Metadata:          metadata,
		MetadataDirective: types.MetadataDirectiveReplace, // IMPORTANT
	})
	if err != nil {
		return fmt.Errorf("in UpdateMetadataR2:\n%w", err)
	}

	return nil

}

func GetObjectR2(ctx context.Context, bucket string, key string) ([]byte, map[string]string, error) {

	if s3Client == nil {
		return nil, nil, fmt.Errorf("could not init s3Client")
	}

	result, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("in GetObjectR2:\n%w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("in GetObjectR2:\n%w", err)
	}

	return data, result.Metadata, nil

}

func PurgeCacheCDN(ctx context.Context, urls []string) error {

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
				Origin: Origin,
			},
		}
	}
	payloadBytes, err := json.Marshal(&payload)
	if err != nil {
		return fmt.Errorf("in PurgeCacheCDN:\n%w", err)
	}

	apiUrl := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/purge_cache", os.Getenv("CF_ZONE_ID"))
	req, err := http.NewRequestWithContext(ctx, "POST", apiUrl, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("in PurgeCacheCDN:\n%w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("CF_API_TOKEN")))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("in PurgeCacheCDN:\n%w", err)
	}
	defer res.Body.Close()

	return nil

}
