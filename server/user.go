package server

import (
	"net/http"
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
