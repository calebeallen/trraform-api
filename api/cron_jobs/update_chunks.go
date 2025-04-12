package cronjobs

import (
	"errors"
	"fmt"
	"net/http"
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
	//get
	var updatedChunkIds []string

	for _, chunkId := range needsUpdate {

		// pull chunk from r2, decode into chunk map
		var chunk map[uint64][]byte
		chunkBytes, err := utils.GetObjectR2(ctx, "chunks", chunkId+".dat")
		var noSuchKey *types.NoSuchKey
		if err != nil && errors.As(err, &noSuchKey) {
			chunk = make(map[uint64][]byte)
		} else if err != nil {
			utils.LogErrorDiscord("UpdateChunks", err, nil)
			continue
		} else {
			chunk, err = plotutils.DecodeChunk(chunkBytes)
			if err != nil {
				utils.LogErrorDiscord("UpdateChunks", err, nil)
				continue
			}
		}

		// get plot ids that need update
		plotIds, err := utils.RedisCli.SMembers(ctx, fmt.Sprintf("updatechunk:%s", chunkId)).Result()
		if err != nil {
			utils.LogErrorDiscord("UpdateChunks", err, nil)
			continue
		}

		// for each plot, pull it's data from plots bucket and set it in the chunk map
		for _, plotIdStr := range plotIds {

			plotId, err := plotutils.PlotIdFromHexString(plotIdStr)
			if err != nil {
				utils.LogErrorDiscord("UpdateChunks", err, nil)
				continue
			}

			plotDataBytes, err := utils.GetObjectR2(ctx, "plots", plotId.ToString()+".dat")
			if err != nil {
				utils.LogErrorDiscord("UpdateChunks", err, nil)
				continue
			}

			chunk[plotId.Id] = plotDataBytes

		}

		// encode chunk and upload it
		newChunkBuf := plotutils.EncodeChunk(chunk)
		err = utils.PutObjectR2(ctx, "chunks", chunkId+".dat", newChunkBuf, "application/octet-stream")
		if err != nil {
			utils.LogErrorDiscord("UpdateChunks", err, nil)
			continue
		}

		// delete plot id set for chunk
		_, err = utils.RedisCli.Del(ctx, fmt.Sprintf("updatechunk:%s", chunkId)).Result()
		if err != nil {
			utils.LogErrorDiscord("UpdateChunks", err, nil)
			continue
		}

		updatedChunkIds = append(updatedChunkIds, chunkId)

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

	utils.MakeAPIResponse(w, r, http.StatusOK, nil, "Success", true)

}
