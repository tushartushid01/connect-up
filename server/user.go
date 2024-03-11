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
