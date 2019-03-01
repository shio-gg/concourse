package resourceserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/google/jsonapi"
	"github.com/tedsuo/rata"
)

func (s *Server) CheckResource(dbPipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("check-resource")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := rata.Param(r, "resource_name")

		var reqBody atc.CheckRequestBody
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		if err != nil {
			logger.Info("malformed-request", lager.Data{"error": err.Error()})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		scanner := s.scannerFactory.NewResourceScanner(dbPipeline)

		err = scanner.ScanFromVersion(logger, resourceName, map[atc.Space]atc.Version{reqBody.Space: reqBody.From})
		switch scanErr := err.(type) {
		case atc.ErrResourceScriptFailed:
			checkResponseBody := atc.CheckResponseBody{
				ExitStatus: scanErr.ExitStatus,
				Stderr:     scanErr.Stderr,
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			err = json.NewEncoder(w).Encode(checkResponseBody)
			if err != nil {
				logger.Error("failed-to-encode-check-response-body", err)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
			}
		case db.ResourceNotFoundError:
			w.WriteHeader(http.StatusNotFound)
		case db.ResourceTypeNotFoundError:
			w.Header().Set("Content-Type", jsonapi.MediaType)
			w.WriteHeader(http.StatusBadRequest)
			jsonapi.MarshalErrors(w, []*jsonapi.ErrorObject{{
				Title:  "Resource Type Not Found Error",
				Detail: err.Error(),
				Status: "400",
			}})
		case error:
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		default:
			w.WriteHeader(http.StatusOK)
		}
	})
}
