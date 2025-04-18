package cronjobs

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
	"trraformapi/utils"
	plotutils "trraformapi/utils/plot_utils"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func UpdateChunks(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	//TODO verify request

	//get chunks that need update
	needsUpdate, err := utils.RedisCli.SMembers(ctx, "needsupdate").Result()
	if err != nil {
		utils.LogErrorDiscord("UpdateChunks", err, nil)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	//for each chunk, get the plot ids that need update
	//pull the chunk from r2
	const concurrencyLimit = 10
	sem := make(chan struct{}, concurrencyLimit)

	var wg sync.WaitGroup
	var updatedIdsMu sync.Mutex
	var updatedChunkIds []string

	for _, _chunkId := range needsUpdate {

		sem <- struct{}{}
		wg.Add(1)

		go func(chunkId string) {

			defer wg.Done()
			defer func() { <-sem }()

			// pull chunk from r2, decode into chunk map
			var chunk map[uint64][]byte
			chunkBytes, _, err := utils.GetObjectR2(ctx, "chunks", chunkId+".dat")
			var noSuchKey *types.NoSuchKey
			if err != nil && errors.As(err, &noSuchKey) {
				chunk = make(map[uint64][]byte)
			} else if err != nil {
				utils.LogErrorDiscord("UpdateChunks", err, nil)
				return
			} else {
				chunk, err = plotutils.DecodeChunk(chunkBytes)
				if err != nil {
					utils.LogErrorDiscord("UpdateChunks", err, nil)
					return
				}
			}

			// get plot ids that need update
			plotIds, err := utils.RedisCli.SMembers(ctx, fmt.Sprintf("updatechunk:%s", chunkId)).Result()
			if err != nil {
				utils.LogErrorDiscord("UpdateChunks", err, nil)
				return
			}

			// for each plot, pull it's data from plots bucket and set it in the chunk map
			for _, plotIdStr := range plotIds {

				plotId, err := plotutils.PlotIdFromHexString(plotIdStr)
				if err != nil {
					utils.LogErrorDiscord("UpdateChunks", err, nil)
					continue
				}

				plotDataBytes, metadata, err := utils.GetObjectR2(ctx, "plots", plotId.ToString()+".dat")
				if err != nil {
					utils.LogErrorDiscord("UpdateChunks", err, nil)
					continue
				}

				isVerified := false
				val, ok := metadata["verified"]
				if ok {
					isVerified, err = strconv.ParseBool(val)
					if err != nil {
						isVerified = false
					}
				}

				//update verified status
				plotData, err := plotutils.Decode(plotDataBytes)
				if err != nil {
					utils.LogErrorDiscord("UpdateChunks", err, nil)
					continue
				}
				plotData.Verified = isVerified
				plotDataBytes, err = plotData.Encode()
				if err != nil {
					utils.LogErrorDiscord("UpdateChunks", err, nil)
					continue
				}

				chunk[plotId.Id] = plotDataBytes

			}

			// encode chunk and upload it
			newChunkBuf := plotutils.EncodeChunk(chunk)
			err = utils.PutObjectR2(ctx, "chunks", chunkId+".dat", newChunkBuf, "application/octet-stream", nil)
			if err != nil {
				utils.LogErrorDiscord("UpdateChunks", err, nil)
				return
			}

			// delete plot id set for chunk
			_, err = utils.RedisCli.Del(ctx, fmt.Sprintf("updatechunk:%s", chunkId)).Result()
			if err != nil {
				utils.LogErrorDiscord("UpdateChunks", err, nil)
				return
			}

			// Synchronized append to updatedChunkIds
			updatedIdsMu.Lock()
			updatedChunkIds = append(updatedChunkIds, chunkId)
			updatedIdsMu.Unlock()

		}(_chunkId)

	}

	wg.Wait()

	//clear cache for chunks
	const ChunkSize = 100
	n := len(updatedChunkIds)
	for i := 0; i < n; i += ChunkSize {

		end := min(i+ChunkSize, n)
		urls := make([]string, end-i)

		for j := i; j < end; j++ {
			urls[j-i] = fmt.Sprintf("https://chunks.trraform.com/%s.dat", updatedChunkIds[j])
		}

		err := utils.PurgeCacheCDN(ctx, urls)
		if err != nil {
			utils.LogErrorDiscord("UpdateChunks", fmt.Errorf("cache purge failed:\n%w", err), nil)
		}
		time.Sleep(time.Second / 7) // cloudflare rate limits > 800 urls/s

	}

	if len(updatedChunkIds) < len(needsUpdate) {

		logData := struct {
			NeedsUpdate []string `json:"needsUpdate"`
			Updated     []string `json:"updated"`
		}{
			NeedsUpdate: needsUpdate,
			Updated:     updatedChunkIds,
		}

		utils.LogErrorDiscord("UpdateChunks", fmt.Errorf("not all chunks updated"), &logData)

	}

	// delete needsupdate set
	_, err = utils.RedisCli.Del(ctx, "needsupdate").Result()
	if err != nil {
		utils.LogErrorDiscord("UpdateChunks", err, nil)
		return
	}

	utils.MakeAPIResponse(w, r, http.StatusOK, nil, "Success", false)

}
