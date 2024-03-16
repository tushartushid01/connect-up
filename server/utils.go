package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/RemoteState/connect-up/connectuperror"
	"github.com/RemoteState/connect-up/models"
	"github.com/RemoteState/connect-up/utils"
	"github.com/gabriel-vasile/mimetype"
	"github.com/sirupsen/logrus"
)

func (srv *Server) getFilterQueries(req *http.Request) (models.UserFilterQueries, error) {
	var filterQueries models.UserFilterQueries
	if req.URL.Query().Get("limit") != "" {
		limit, err := strconv.Atoi(req.URL.Query().Get("limit"))
		if err != nil {
			return models.UserFilterQueries{}, err
		}
		if limit <= 0 {
			limit = 10
		}
		filterQueries.UserLimit = limit
	} else {
		filterQueries.UserLimit = 10
	}

	if req.URL.Query().Get("page") != "" {
		pageNo, err := strconv.Atoi(req.URL.Query().Get("page"))
		if err != nil {
			return models.UserFilterQueries{}, err
		}
		if pageNo < 0 {
			pageNo = 0
		}
		filterQueries.Page = pageNo
	} else {
		filterQueries.Page = 0
	}

	if req.URL.Query().Get("countries") != "" {
		countries := req.URL.Query().Get("countries")
		filterQueries.CountriesName = strings.Split(countries, ",")
	} else {
		filterQueries.CountriesName = make([]string, 0)
	}

	if req.URL.Query().Get("states") != "" {
		states := req.URL.Query().Get("states")
		filterQueries.StatesName = strings.Split(states, ",")
	} else {
		filterQueries.StatesName = make([]string, 0)
	}

	if req.URL.Query().Get("gender") != "" {
		genderQuery := req.URL.Query().Get("gender")
		gender := strings.Split(genderQuery, ",")
		filterQueries.Gender = gender
	} else {
		filterQueries.Gender = make([]string, 0)
	}

	if req.URL.Query().Get("fromAge") != "" {
		fromAge, err := strconv.Atoi(req.URL.Query().Get("fromAge"))
		if err != nil {
			return filterQueries, nil
		}

		filterQueries.FromAge = fromAge
	}

	if req.URL.Query().Get("toAge") != "" {
		toAge, err := strconv.Atoi(req.URL.Query().Get("toAge"))
		if err != nil {
			return filterQueries, nil
		}

		filterQueries.ToAge = toAge
	}

	if req.URL.Query().Get("industries") != "" {
		industriesQueries := req.URL.Query().Get("industries")
		industries := strings.Split(industriesQueries, ",")
		industriesIDs, err := utils.ToIntSliceFromString(industries)
		if err != nil {
			return filterQueries, err
		}
		filterQueries.IndustriesIDs = industriesIDs
	} else {
		filterQueries.IndustriesIDs = make([]int, 0)
	}

	if req.URL.Query().Get("isVerified") != "" {
		isVerifiedQueries := req.URL.Query().Get("isVerified")
		isVerified, err := strconv.ParseBool(isVerifiedQueries)
		if err != nil {
			return filterQueries, err
		}
		filterQueries.IsVerified.Bool = isVerified
		filterQueries.IsVerified.Valid = true
	}

	if req.URL.Query().Get("isCompleted") != "" {
		isCompletedQueries := req.URL.Query().Get("isCompleted")
		isCompleted, err := strconv.ParseBool(isCompletedQueries)
		if err != nil {
			return filterQueries, err
		}
		filterQueries.IsCompleted.Bool = isCompleted
		filterQueries.IsCompleted.Valid = true
	}

	if req.URL.Query().Get("searchText") != "" {
		filterQueries.SearchText = req.URL.Query().Get("searchText")
	}

	return filterQueries, nil
}

func (srv *Server) uploadImagesFromCSV(resp http.ResponseWriter, req *http.Request, createIndustry []models.IndustryDetailsForTestCase) {
	for i := range createIndustry {
		if createIndustry[i].Category == "" {
			connectuperror.RespondClientErr(resp, req, errors.New("category cannot be empty"), http.StatusBadRequest, "category cannot be empty")
			return
		}

		response, err := http.Get(createIndustry[i].URL)
		if err != nil {
			logrus.Errorf("createIndustry: unable to get image from url %v", err)
			return
		}

		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			logrus.Errorf("createIndustry: unable to get image from url %v", err)
			return
		}

		mediaType := strings.Split(mimetype.Detect(bodyBytes).String(), "/")
		if mediaType[0] != string(models.MIMEMediaTypeImage) {
			connectuperror.RespondClientErr(resp, req, err, http.StatusBadRequest, "file is not an image")
			return
		}

		file, err := os.Create("industry")
		if err != nil {
			logrus.Errorf("createIndustry: unable to get image from url %v", err)
			return
		}

		_, err = io.Copy(file, response.Body)
		if err != nil {
			logrus.Errorf("createIndustry: unable to get image from url %v", err)
			return
		}

		data, _ := file.Stat()

		if data.Size()/(1024*1024) > 5 {
			connectuperror.RespondClientErr(resp, req, err, http.StatusBadRequest, "file size cannot be more then 5 mb")
			return
		}

		err = file.Close()
		if err != nil {
			logrus.Errorf("createIndustry:unable to close file %v", err)
		}

		err = response.Body.Close()
		if err != nil {
			logrus.Errorf("createIndustry:unable to close response body %v", err)
		}

		ok := response.Close
		if !ok {
			logrus.Errorf("createIndustry:unable to close response %v", err)
		}

		createIndustry[i].FileName = fmt.Sprintf("%v-%v.svg", time.Now().Unix(), "industry")
		createIndustry[i].FilePath = fmt.Sprintf(`images/%v/%v-%s`, models.UploadBinaryTypeImage, time.Now().Unix(), createIndustry[i].FileName)

		err = srv.StorageProvider.Upload(req.Context(), utils.GetUploadsBucketName(), file, createIndustry[i].FileName, "application/octet-stream", false)
		if err != nil {
			connectuperror.RespondClientErr(resp, req, err, http.StatusBadRequest, "unable to upload image")
			return
		}

		createIndustry[i].UploadURL, err = srv.StorageProvider.GetSharableURL(utils.GetUploadsBucketName(), createIndustry[i].FilePath, time.Hour*24*365)
		if err != nil {
			connectuperror.RespondClientErr(resp, req, err, http.StatusBadRequest, "unable to get upload url")
			return
		}
	}
}
