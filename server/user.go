package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/RemoteState/connect-up/config"
	"github.com/RemoteState/connect-up/connectuperror"
	"github.com/RemoteState/connect-up/models"
	"github.com/RemoteState/connect-up/utils"
	"github.com/sirupsen/logrus"
)

const minDetectionConfidence = 0.5

func (srv *Server) getUserContext(req *http.Request) *models.UserContext {
	return srv.Middlewares.GetUserContext(req)
}

// UserInfo  godoc
// @Summary 	User Info Api
// @Tags 		user info
// @Accept 		json
// @Produce 	json
// @Success     200 {object} models.UserInfo
// @Failure 	500 {object} connectuperror.clientError
// @Router 		/api/user/info [get]
// @Security 	ApiKeyAuth
/*     	* userInfo
* @Description This method is used to get user profile info.
		This will provide all the data of user which is provided by user on creating account.
*/
func (srv *Server) userInfo(resp http.ResponseWriter, req *http.Request) {
	uc := srv.getUserContext(req)
	startTime := time.Now()

	userInfo, err := srv.DBHelper.GetUserInfo(uc.ID)
	if err != nil {
		connectuperror.RespondGenericServerErr(resp, req, err, "Failed to get user information")
		return
	}

	logrus.Infof("userInfo: db end  %d", time.Since(startTime).Milliseconds())
	startTime = time.Now()

	emailVerificationFlows := models.EmailVerificationFlow{
		IsEmailVerificationFlowNeeded: srv.DynamicConfig.GetBool(config.EmailVerificationFlowConfig),
		IsEmailVerificationCompulsory: srv.DynamicConfig.GetBool(config.EmailVerificationCompulsoryConfig),
	}

	phoneVerificationFlows := models.PhoneVerificationFlow{
		IsPhoneVerificationFlowNeeded: srv.DynamicConfig.GetBool(config.PhoneVerificationFlowConfig),
		IsPhoneVerificationCompulsory: srv.DynamicConfig.GetBool(config.PhoneVerificationCompulsoryConfig),
	}

	verificationFlow := models.VerificationFlow{
		PhoneVerificationFlows: phoneVerificationFlows,
		EmailVerificationFlows: emailVerificationFlows,
	}

	userInfo.VerificationFlows = &verificationFlow

	isAllDataAvailable := true
	if !userInfo.Phone.Valid || userInfo.Gender == models.None || !userInfo.DateOfBirth.Valid || !userInfo.Email.Valid || userInfo.Name == "" {
		isAllDataAvailable = false
	}

	isProfileCompleted := true
	if !userInfo.Email.Valid || !userInfo.Phone.Valid || userInfo.Gender == models.None || !userInfo.DateOfBirth.Valid || !userInfo.ProfileImageID.Valid || !userInfo.About.Valid || !userInfo.Headline.Valid || userInfo.LookingFor == "" || !userInfo.CurrentPosition.Valid {
		isProfileCompleted = false
	}

	// if !userInfo.IsFirstTimeRegistered {
	//	userInfo.IsFirstTimeRegistered = true
	// } else {
	//	userInfo.IsFirstTimeRegistered = false
	// }

	userInfo.IsAllDataAvailable = isAllDataAvailable
	userInfo.IsProfileCompleted = isProfileCompleted

	utils.EncodeJSON200Body(resp, userInfo)
	logrus.Infof("userInfo: encoding end  %d", time.Since(startTime).Milliseconds())
}

/*
  - emailVerification
  - @Description This method is used to verify the email.
    This method is used when user confirm the email by clicking
    the link provided on email. This method will verify the email
    for that user.
*/
func (srv *Server) emailVerification(resp http.ResponseWriter, req *http.Request) {
	uc := srv.getUserContext(req)

	userInfo, err := srv.DBHelper.GetUserInfo(uc.ID)
	if err != nil {
		connectuperror.RespondGenericServerErr(resp, req, err, "error in getting user info")
		return
	}

	if userInfo.EmailVerifiedAt.Valid {
		connectuperror.RespondClientErr(resp, req, errors.New("email already verified"), http.StatusBadRequest, "You are already Verified, Please reopen the app")
		return
	}

	emailLimit := srv.DynamicConfig.GetInt(config.EmailLimitConfig)
	if emailLimit == 0 {
		emailLimit = 120
	}

	count, convErr := srv.RateLimitCachedValue(string(models.EmailLimitCacheKey))
	if convErr != nil {
		connectuperror.RespondGenericServerErr(resp, req, convErr, "unable get value from cache")
		return
	}

	if count > emailLimit {
		connectuperror.RespondClientErr(resp, req, errors.New("too many attempts"), http.StatusTooManyRequests, "too many attempts", "too many attempts")
		return
	}

	err = srv.DBHelper.ExpireAllVerificationLink(uc.ID, userInfo.Email.String)
	if err != nil {
		logrus.Errorf("emailVerification: error in updating email verification link")
		return
	}

	uniqueToken, err := srv.DBHelper.SendEmailVerificationLink(uc.ID, userInfo.Email.String)
	if err != nil {
		logrus.Errorf("emailVerification: error in sending email verification link")
		return
	}

	userIDs := make([]int, 0)
	userIDs = append(userIDs, userInfo.ID)
	emailTemplate, err := srv.EmailProvider.GetEmailTemplate(models.EmailTypeVerifyUsingLink, userIDs)
	if err != nil {
		logrus.Errorf("emailVerification: error in getting email template")
		return
	}

	var verificationLink string

	if env.IsDev() {
		verificationLink = fmt.Sprintf("https://dev.connectup.com/verify/%s/email", uniqueToken)
	} else if env.IsMain() {
		verificationLink = fmt.Sprintf("https://connectup.com/verify/%s/email", uniqueToken)
	}

	emailTemplate.DynamicData["verificationLink"] = verificationLink

	err = srv.EmailProvider.Send(emailTemplate)
	if err != nil {
		logrus.Errorf("emailVerification: error in sending verification email")
		return
	}

	utils.EncodeJSON200Body(resp, map[string]interface{}{
		"message": "success",
	})
}

func (srv *Server) verifyEmail(resp http.ResponseWriter, req *http.Request) {
	var verifyEmail models.VerifyEmail
	err := json.NewDecoder(req.Body).Decode(&verifyEmail)
	if err != nil {
		connectuperror.RespondClientErr(resp, req, err, http.StatusBadRequest, "Unable to edit user settings", "error parsing request")
		return
	}

	isValidAndUserID, err := srv.DBHelper.VerifyEmailVerificationLink(verifyEmail.Token)
	if err != nil {
		if err == sql.ErrNoRows {
			connectuperror.RespondClientErr(resp, req, err, http.StatusBadRequest, "invalid token", "invalid token")
			return
		}
		logrus.Errorf("verifyEmail: error in verifying email verification link")
		return
	}
	if !isValidAndUserID.IsValid {
		connectuperror.RespondClientErr(resp, req, err, http.StatusBadRequest, "token expired", "token expired")
		return
	}
	if isValidAndUserID.IsValid {
		go func() {
			err := srv.DBHelper.SetEmailTokenIsVerified(verifyEmail.Token)
			if err != nil {
				connectuperror.RespondGenericServerErr(resp, req, err, "Failed to update user information")
			}
		}()
		go func() {
			err := srv.DBHelper.SetUserEmailTokenIsVerified(isValidAndUserID.UserID)
			if err != nil {
				connectuperror.RespondGenericServerErr(resp, req, err, "Failed to update user information")
			}
		}()
		go func() {
			err := srv.NotificationProvider.SendPushNotificationForEmailVerification(isValidAndUserID.UserID)
			if err != nil {
				connectuperror.RespondGenericServerErr(resp, req, err, "Failed to send silent notification for email verification")
			}
		}()
	}

	utils.EncodeJSON200Body(resp, map[string]interface{}{
		"message": "success",
	})
}

/*     	* getAllIndustriesForUser
* 	@Description This method is used to get list of all the industries
			for user. This method is used by user to completing the self
			profile.
*/

func (srv *Server) getAllIndustriesForUser(resp http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	category := req.URL.Query().Get("category")

	if category == "" {
		category = string(models.IndustriesCategoryConnectionsAndGroups)
	}

	industries, err := srv.DBHelper.GetAllIndustriesForUser(category)
	if err != nil {
		connectuperror.RespondGenericServerErr(resp, req, err, "Failed to get user information")
		return
	}

	utils.EncodeJSON200Body(resp, industries)
	logrus.Infof("getAllIndustriesForUser: request time for all industries for user successfully: %d", time.Since(startTime).Milliseconds())
}

func (srv *Server) getAllIndustries(resp http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	filterQueries, err := utils.GetIndustriesFilters(req)
	if err != nil {
		connectuperror.RespondClientErr(resp, req, err, http.StatusBadRequest, "errors in getting filters")
		return
	}

	industries, err := srv.DBHelper.GetAllIndustries(filterQueries)
	if err != nil {
		connectuperror.RespondGenericServerErr(resp, req, err, "Failed to get user information")
		return
	}

	utils.EncodeJSON200Body(resp, industries)
	logrus.Infof("getAllIndustries: request time for all industries for user successfully: %d", time.Since(startTime).Milliseconds())
}

/*     	* ping
* 	@Description This method is used to update the online status of user.
 */
func (srv *Server) ping(resp http.ResponseWriter, req *http.Request) {
	uc := srv.getUserContext(req)

	err := srv.DBHelper.Ping(uc)
	if err != nil {
		connectuperror.RespondGenericServerErr(resp, req, err, "Failed to ping user")
		return
	}

	utils.EncodeJSON200Body(resp, map[string]interface{}{
		"message": "pong",
	})
}

/*     	* getOnlineStatusOfUsers
* 	@Description This method is used to get the online status of other user.
 */
func (srv *Server) getOnlineStatusOfUsers(resp http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	var users struct {
		UserIDs []int `json:"userIds"`
	}

	err := json.NewDecoder(req.Body).Decode(&users)
	if err != nil {
		connectuperror.RespondClientErr(resp, req, err, http.StatusBadRequest, "Failed to get status of users", "unable to parse")
		return
	}

	statuses, err := srv.DBHelper.GetOnlineStatusOfUsers(users.UserIDs)
	if err != nil {
		connectuperror.RespondGenericServerErr(resp, req, err, "Failed to get user online statuses")
		return
	}

	utils.EncodeJSON200Body(resp, map[string]interface{}{
		"statuses": statuses,
	})
	logrus.Infof("getAllIndustries: request time for all industries for user successfully: %d", time.Since(startTime).Milliseconds())
}

/*     	* upload
* @Description This method is used to upload image in db and cloud storage.
 */
func (srv *Server) upload(resp http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	uc := srv.getUserContext(req)

	defer func(req *http.Request) {
		if req.MultipartForm != nil { // prevent panic from nil pointer
			if err := req.MultipartForm.RemoveAll(); err != nil {
				logrus.Errorf("Unable to remove all multipart form. %+v", err)
			}
		}
	}(req)

	req.Body = http.MaxBytesReader(resp, req.Body, 51<<20)

	if err := req.ParseMultipartForm(51 << 20); err != nil {
		if err == io.EOF || err.Error() == string(models.MultipartUnexpectedEOF) {
			logrus.Warn("EOF")
		} else {
			logrus.Errorf("[ParseMultipartForm] %s", err.Error())
		}
		connectuperror.RespondClientErr(resp, req, err, http.StatusBadRequest, "unable to parse file", "error parsing file")
		return
	}

	file, header, err := req.FormFile("file")

	defer func() {
		if err = file.Close(); err != nil {
			logrus.Errorf("Unable to close file multipart form. %+v", err)
		}
	}()

	if err != nil {
		if err == io.EOF {
			logrus.Warn("EOF")
		} else {
			logrus.Error(err)
		}

		connectuperror.RespondClientErr(resp, req, err, http.StatusBadRequest, "unable to read file", "unable to read file")
		return
	}

	typeOfUploadBinary := models.UploadBinaryType(req.FormValue("upload_binary_type"))
	typeOfUpload := models.UploadType(req.FormValue("type"))

	filePath := ""

	switch typeOfUploadBinary {
	case models.UploadBinaryTypeImage:
		filePath = fmt.Sprintf(`images/%v/%v-%s`, typeOfUpload, time.Now().Unix(), header.Filename)
	case models.UploadBinaryTypeVideo:
		filePath = fmt.Sprintf(`videos/%v/%v-%s`, typeOfUpload, time.Now().Unix(), header.Filename)
	case models.UploadBinaryTypeAudio:
		filePath = fmt.Sprintf(`audios/%v/%v-%s`, typeOfUpload, time.Now().Unix(), header.Filename)
	case models.UploadBinaryTypeDocument:
		filePath = fmt.Sprintf(`documents/%v/%v-%s`, typeOfUpload, time.Now().Unix(), header.Filename)
	default:
		connectuperror.RespondGenericServerErr(resp, req, errors.New("file type not valid"), "invalid file type")
		return
	}

	logrus.Infof("current branch : %v", os.Getenv("BRANCH"))
	err = srv.StorageProvider.Upload(req.Context(), utils.GetUploadsBucketName(), file, filePath, "application/octet-stream", false)
	if err != nil {
		connectuperror.RespondGenericServerErr(resp, req, err, "unable to upload file")
		return
	}

	url, err := srv.StorageProvider.GetSharableURL(utils.GetUploadsBucketName(), filePath, time.Hour*24*365)
	if err != nil {
		connectuperror.RespondGenericServerErr(resp, req, err, "unable to get url")
		return
	}

	SQL := `INSERT INTO uploads 
			(name, bucket, path, type, uploaded_by, binary_type, url, url_expiration_time) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id,u_id
			`

	var files models.Upload

	args := []interface{}{
		header.Filename,
		utils.GetUploadsBucketName(),
		filePath,
		typeOfUpload,
		uc.ID,
		typeOfUploadBinary,
		url,
		time.Now().AddDate(1, 0, 0),
	}

	err = srv.PSQL.DB().Get(&files, SQL, args...)
	if err != nil {
		logrus.Errorf("upload: error inserting into upload: %v", err)
		connectuperror.RespondGenericServerErr(resp, req, err, "Error inserting file")
		return
	}

	var thumbURL string
	if typeOfUploadBinary == models.UploadBinaryTypeVideo {
		if thumbURL, err = thumbnailUpload(req, srv, url, header, uc, files); err != nil {
			logrus.Errorf("upload: unable to generate thumbnail %v", err)
		}
	}

	if strings.Contains(header.Filename, ".svg") {
		_, err := srv.ConvertSVGToPNG(resp, req, file, typeOfUpload, uc, files)
		if err != nil {
			logrus.Errorf("uploadV2: unable to generate png of svg thumbnail %v", err)
		}
	}

	if typeOfUploadBinary != models.UploadBinaryTypeVideo {
		thumbURL = url
	}

	utils.EncodeJSON200Body(resp, map[string]interface{}{
		"id":           files.FileID,
		"imageUID":     files.FileUUID,
		"url":          url,
		"thumbnailUrl": thumbURL,
	})
	logrus.Infof("upload: request time upload data successfully: %d", time.Since(startTime).Milliseconds())
}

func (srv *Server) createUserSession(resp http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	uc := srv.getUserContext(req)

	var createSessionRequest models.CreateSessionRequest
	if err := json.NewDecoder(req.Body).Decode(&createSessionRequest); err != nil {
		connectuperror.RespondClientErr(resp, req, err, http.StatusBadRequest, "Failed to create session")
		return
	}

	if createSessionRequest.Platform == "android" || createSessionRequest.Platform == "ios" {
		isValid, err := srv.DBHelper.CheckIfSessionAlreadyRunning(&createSessionRequest)
		if err != nil {
			connectuperror.RespondClientErr(resp, req, err, http.StatusBadRequest, "failed to check session is valid")
			return
		}

		if !isValid {
			connectuperror.RespondClientErr(resp, req, errors.New("not valid"), http.StatusBadRequest, "Another session is running on this device")
			return
		}
	}
	createSessionRequest.AuthID = uc.AuthID
	newSessionToken, err := srv.DBHelper.StartNewSession(uc.ID, &createSessionRequest)
	if err != nil {
		connectuperror.RespondClientErr(resp, req, err, http.StatusBadRequest, "failed to create new session")
		return
	}

	utils.EncodeJSON200Body(resp, newSessionToken)
	logrus.Infof("createUserSession: request time for all industries for user successfully: %d", time.Since(startTime).Milliseconds())
}

func (srv *Server) validateUserSession(resp http.ResponseWriter, req *http.Request) {
	uc := srv.getUserContext(req)

	if uc.Session == nil {
		connectuperror.RespondClientErr(resp, req, errors.New("session not found"), http.StatusBadRequest, "session not found")
		return
	}

	newSessionID, err := srv.DBHelper.ValidateSession(uc.Session.Token)
	if err != nil {
		connectuperror.RespondClientErr(resp, req, err, http.StatusBadRequest, "failed to create new session")
		return
	}

	utils.EncodeJSON200Body(resp, newSessionID)
}

func (srv *Server) endUserSession(resp http.ResponseWriter, req *http.Request) {
	uc := srv.getUserContext(req)
	deviceID := req.Header.Get("deviceID")
	logrus.Infof("endUserSession: end session for userId: %v and session token is: %v and deviceID is: %v", uc.ID, uc.Session.Token, deviceID)

	logrus.Infof("endUserSession: end session for userId: %v and session token is: %v", uc.ID, uc.Session.Token)

	if uc.Session == nil {
		connectuperror.RespondClientErr(resp, req, errors.New("not session found"), http.StatusBadRequest, "session not found")
		return
	}

	err := srv.DBHelper.EndSession(uc.Session.Token)
	if err != nil {
		connectuperror.RespondClientErr(resp, req, err, http.StatusBadRequest, "failed to end session")
		return
	}

	srv.CacheProvider.ClearCache(srv.CacheProvider.GenerateKey(userContextCacheKey, uc.AuthID, uc.Session.Token))

	utils.EncodeJSON200Body(resp, map[string]interface{}{
		"message": "success",
	})
}

/*
  - deleteUser
  - @Description This method is used to delete user profile
    from the server and firebase.
*/
func (srv *Server) deleteUser(resp http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	uc := srv.getUserContext(req)

	err := srv.DBHelper.DeleteUser(uc.ID)
	if err != nil {
		connectuperror.RespondGenericServerErr(resp, req, err, "unable to delete user")
		return
	}
	logrus.Infof("deleteUser: request time after deleting user from db: %d", time.Since(startTime).Milliseconds())

	// Delete authId from Firebase
	err = srv.AuthProvider.DeleteAuthUser(req.Context(), uc.AuthID)
	if err != nil {
		connectuperror.RespondGenericServerErr(resp, req, err, "Error while deleting auth user")
		return
	}

	logrus.Infof("deleteUser: request time after deleting user from firbase: %d", time.Since(startTime).Milliseconds())

	go func() {
		// deleting user from the group
		err = srv.DBHelper.DeleteGroupsForUser(uc.ID)
		if err != nil {
			logrus.Errorf("DeleteUser: unable to delete from user from groups %v", err)
			connectuperror.RespondGenericServerErr(resp, req, err, "Error while deleting groups of user")
			return
		}
	}()

	go func() {
		// making new admin for chatGroup
		err := srv.DBHelper.CreateNewChatGroupAdmin(uc.ID)
		if err != nil {
			logrus.Errorf("Error updating new admin: %v", err)
		}
	}()

	srv.CacheProvider.ClearCache(srv.CacheProvider.GenerateKey(userContextCacheKey, uc.AuthID, uc.Session.Token))

	utils.EncodeJSON200Body(resp, map[string]interface{}{
		"message": "success",
	})
	logrus.Infof("deleteUser: request time after success fully deleting: %d", time.Since(startTime).Milliseconds())
}

func (srv *Server) deleteUserByAdmin(resp http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	userID, err := strconv.Atoi(chi.URLParam(req, "userID"))
	if err != nil {
		connectuperror.RespondClientErr(resp, req, err, http.StatusBadRequest, "Error parsing userId")
		return
	}

	authID, err := srv.DBHelper.GetAuthTokenByID(userID)
	if err != nil {
		connectuperror.RespondGenericServerErr(resp, req, err, "unable to get auth token by id")
		return
	}

	err = srv.DBHelper.DeleteUser(userID)
	if err != nil {
		connectuperror.RespondGenericServerErr(resp, req, err, "unable to delete user")
		return
	}

	logrus.Infof("deleteUserByAdmin: request time after deleting user from db by admin: %d", time.Since(startTime).Milliseconds())

	// Delete authId from Firebase
	err = srv.AuthProvider.DeleteAuthUser(req.Context(), authID)
	if err != nil {
		connectuperror.RespondGenericServerErr(resp, req, err, "Error while deleting auth user")
		return
	}

	logrus.Infof("deleteUserByAdmin: request time after deleting user from firbase: %d", time.Since(startTime).Milliseconds())

	go func() {
		// deleting user from the group
		err = srv.DBHelper.DeleteGroupsForUser(userID)
		if err != nil {
			logrus.Errorf("deleteUserByAdmin: unable to delete from user from groups: %v", err)
			connectuperror.RespondGenericServerErr(resp, req, err, "Error while deleting groups of user")
			return
		}
		err = srv.DBHelper.EndSessionOfUserByAdmin(userID, authID)
		if err != nil {
			connectuperror.RespondGenericServerErr(resp, req, err, "Error while deleting session of user")
			return
		}
	}()

	// srv.CacheProvider.ClearCache(srv.CacheProvider.GenerateKey(userContextCacheKey, uc.AuthID, uc.Session.Token))

	utils.EncodeJSON200Body(resp, map[string]interface{}{
		"message": "success",
	})
	logrus.Infof("deleteUserByAdmin: request time after success fully deleting user by admin: %d", time.Since(startTime).Milliseconds())
}
