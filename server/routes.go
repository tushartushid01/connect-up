package server

import (
	"github.com/connect-up/models"
	"github.com/go-chi/chi"
	httpSwagger "github.com/swaggo/http-swagger"
)

func (srv *Server) InjectRoutes() *chi.Mux {
	r := chi.NewRouter()
	r.Route("/docs", func(docs chi.Router) {
		docs.Get("/*", httpSwagger.Handler())
	})
	r.Get(`/health`, srv.HealthCheck)

	r.Route("/api", func(api chi.Router) {
		api.Use(srv.Middlewares.APITimeMiddleware()...)
		// api.Use(srv.Middlewares.GetClientRealIP()...)
		api.Use(srv.Middlewares.Default()...)
		// api.Use(srv.Middlewares.RequestResponseLoggerMiddleware()...)
		api.Get(`/health`, srv.HealthCheck)
		api.Route("/", func(public chi.Router) {
			public.Post("/register", srv.createNewUser)
			public.Post("/register_v3", srv.createNewUserV3)
			public.Post("/login", srv.login)
			public.Post("/login_v2", srv.loginV2)
			public.Post("/login_v3", srv.loginV3)
			// public.Post("/reset_password_email", srv.resetPassword)
			public.Post("/send_otp", srv.sendOTPRequest)
			public.Post("/feedback", srv.sendFeedback)
			public.Post("/verify_email_link", srv.verifyEmail)
			public.Post("/verify_otp", srv.verifyOTPForResetPassword(models.OTPReasonTypeResetPassword))
			public.Post("/change_password_using_otp", srv.changePasswordUsingOTP)
			public.Post("/pn", srv.SendPushNotification)
			public.Post("/psn", srv.SendPushNotificationV2)
			public.Get("/attachment/{attachmentID}", srv.getAttachment)
			public.Post("/ios-pn", srv.sendTestPushNotification)
			public.Get("/faqs", srv.frequentlyAskedQuestions)

			public.Route("/{id}", func(local chi.Router) {
				local.Use(srv.Middlewares.CheckLocalEnv()...)
				local.Get("/", srv.getAuthToken)
			})

			public.Route("/test", func(testCase chi.Router) {
				testCase.Use(srv.Middlewares.CheckLocalEnv()...)
				testCase.Post("/create", srv.createAdmin)
				testCase.Route("/delete", func(testAction chi.Router) {
					testAction.Use(srv.Middlewares.AUTH()...)
					// testAction.Use(srv.Middlewares.UserAdminCheck(models.RoleAdmin)...)
					testAction.Delete("/", srv.deleteUser)
				})
			})

			public.Route("/user", func(user chi.Router) {
				user.Use(srv.Middlewares.AUTH()...)
				// user.Use(srv.Middlewares.APITimeMiddleware()...)
				user.Delete("/", srv.deleteUser)
				user.Get("/info", srv.userInfo)
				user.Post("/send_verification_email", srv.emailVerification)

				user.Put("/phone", srv.changePhone)
				user.Put("/email", srv.changeEmail)
				user.Put("/skip_phone", srv.skipPhone)
				user.Get("/settings", srv.userSettings)
				user.Post("/settings", srv.upsertUserSettings)
				user.Post("/change_password", srv.changePassword)
				user.Post("/blocked_contacts", srv.editBlockedContacts)
				user.Get("/blocked_contacts", srv.getBlockedContacts)
				user.Get("/blocked_contactsV2", srv.getBlockedContactsV2)
				user.Post("/toggle_block", srv.toggleBlock)
				user.Post("/upload_image", srv.upload)
				user.Post("/upload_image_v3", srv.uploadImageV3)
				user.Get("/png", srv.getPng)
				user.Post("/upload_image_v2", srv.uploadV2)
				user.Get("/industries", srv.getAllIndustriesForUser)
				user.Post("/industries", srv.addIndustries)
				user.Post("/ping", srv.ping)
				user.Post("/online_status", srv.getOnlineStatusOfUsers)
				user.Post("/user_rating", srv.addUserRating)
				user.Post("/location", srv.addNewUserLocation)
				user.Post("/verify_phone_otp", srv.verifyOTP(models.OTPReasonTypeVerifyPhone))
				user.Post("/verify_email_otp", srv.verifyOTP(models.OTPReasonTypeVerifyEmail))
				user.Get("/notifications", srv.getNotifications)
				user.Put("/read_notification", srv.readNotification)
				user.Get("/notifications_count", srv.getUnreadNotificationsCount)
				user.Get("/connections_count", srv.totalBlockedContactsAndConnections)
				user.Post("/decline_call", srv.declineCall)
				user.Post("/decline_call_v2", srv.declineCallV2)
				user.Post("/end_call", srv.endCall)

				user.Route("/ws", func(ws chi.Router) {
					ws.Get("/chat", srv.connect)
					ws.Get("/realtime", srv.realTimeWS)
				})

				user.Route("/session", func(session chi.Router) {
					session.Post("/", srv.createUserSession)
					session.Get("/", srv.validateUserSession) // Currently, not in use
					session.Put("/end", srv.endUserSession)
					session.Put("/fcm", srv.updateFCMToken)
					session.Put("/voip_token", srv.updateVoipToken)
				})

				user.Route("/profile", func(profile chi.Router) {
					profile.Get("/", srv.getSelfProfileDetails)
					profile.Get("/{userID}", srv.getOtherUserProfileDetails)
					profile.Put("/", srv.editProfile)
					profile.Put("/image", srv.updateProfileImage)
				})

				user.Get("/connections/all", srv.getAllConnections)
				user.Get("/connections_list", srv.getAllConnectionList)
				user.Post("/match_event", srv.matchEvent)
				user.Get("/recommendations", srv.recommendations)
				user.Get("/total_recommendations", srv.totalRecommendations)
				user.Post("/undo_recommendation", srv.undoRecommendation)
				user.Post("/all_recommendations", srv.allRecommendations)
				user.Post("/decline_recommendation", srv.declineUserRecommendation)
				user.Get("/report_type", srv.reportType)
				user.Route("/connection", func(connection chi.Router) {
					connection.Route("/request", func(request chi.Router) {
						request.Get("/all", srv.allRequests)
						request.Post("/send", srv.sendConnectionRequest)
						request.Get("/inbound", srv.InboundRequests)
						request.Get("/inboundV2", srv.InboundRequestsV2)
						request.Get("/outbound", srv.OutboundRequests)
						request.Get("/outboundV2", srv.OutboundRequestsV2)
						request.Put("/status", srv.updateConnectionRequestStatus)
						request.Put("/remove", srv.removeUserConnection)
					})
				})
			})

			public.Route("/chat", func(chat chi.Router) {
				chat.Use(srv.Middlewares.AUTH()...)
				// chat.Use(srv.Middlewares.APITimeMiddleware()...)

				chat.Route("/chat_group", func(chatGroups chi.Router) {
					chatGroups.Get("/", srv.getAllChatGroups)
					chatGroups.Post("/", srv.createNewChatGroup)

					chatGroups.Route("/{chatGroupId}", func(chatGroup chi.Router) {
						chatGroup.Use(srv.Middlewares.ChatGroupMemberCheck()...)
						chatGroup.Get("/details", srv.getChatGroupDetails)
						chatGroup.Put("/toggle_notification", srv.toggleMuteNotification)
						chatGroup.Get("/available_room", srv.getAvailableTwilioRoom)
						chatGroup.Get("/join_room", srv.joinRoom)
						chatGroup.Post("/leave", srv.leaveChatGroup)
						chatGroup.Post("/notification", srv.toggleNotifications)

						chatGroup.Route("/admin", func(chatGroupAdmin chi.Router) {
							chatGroupAdmin.Use(srv.Middlewares.ChatGroupAdminCheck()...)
							chatGroupAdmin.Delete("/", srv.deleteChatGroup)
							chatGroupAdmin.Put("/", srv.editChatGroup)
							chatGroupAdmin.Post("/toggle_admins", srv.toggleGroupAdmins)
							chatGroupAdmin.Post("/set_admin", srv.setAdmins)
							chatGroupAdmin.Post("/edit_participants", srv.editParticipantsInChatGroup)
						})

						chatGroup.Route("/message", func(message chi.Router) {
							message.Get("/", srv.getAllMessages)
							message.Get("/after_time", srv.getAllMessagesAfterTimestamp)
							message.Get("/{attachmentId}", srv.getMessageAttachment)
							message.Post("/", srv.deleteMessages)
							message.Delete("/clear_all", srv.clearAllMessages)
						})
					})
				})
			})
			public.Route("/admin", func(admin chi.Router) {
				admin.Post("/login", srv.loginAdmin)

				admin.Route("/", func(admin chi.Router) {
					admin.Use(srv.Middlewares.AUTH()...)
					admin.Use(srv.Middlewares.UserAdminCheck(models.RoleAdmin)...)
					// admin.Use(srv.Middlewares.APITimeMiddleware()...)

					admin.Delete("/", srv.deleteUser)
					admin.Get("/report_type", srv.reportType)
					admin.Post("/broadcast", srv.broadcastMessage)
					admin.Post("/cover-image", srv.updateDefaultCoverImage)
					admin.Get("/category", srv.industryCategories)
					admin.Post("/report_type", srv.createReportType)
					admin.Get("/broadcasts", srv.broadCastHistory)
					admin.Get("/country", srv.getCountryWithCountryCode)
					admin.Get("/broadcast/{broadcastID}", srv.getBroadcastMessageDetail)
					admin.Route("/users", func(users chi.Router) {
						users.Get("/", srv.getUsersList)
						users.Get("/downloads", srv.downloadUsersList)
						users.Get("/filters", srv.userFilters)

						users.Route("/{userID}", func(users chi.Router) {
							users.Get("/", srv.getUserProfileDetails)
							users.Post("/suspend_user", srv.suspendUser)
							users.Put("/", srv.updateUserDetails)
							users.Delete("/", srv.deleteUserByAdmin)
						})
					})

					admin.Route("/dashboard", func(dashboard chi.Router) {
						dashboard.Get("/details", srv.dashboardDetails)
						dashboard.Route("/charts", func(charts chi.Router) {
							charts.Get("/", srv.getUserChartData)
							charts.Get("/industry_user_count", srv.getTopIndustries)
							charts.Get("/country_user_count", srv.getCountryWiseUsersCount)
							charts.Get("/country_active_user_count", srv.getCountryWiseActiveUsers)
							charts.Get("/state_user_count", srv.getStateWiseUsersCount)
							charts.Get("/state_active_user_count", srv.getStateWiseActiveUsers)
							charts.Get("/most_active_time_user", srv.getMostActiveTimeUser)
						})
					})

					admin.Route("/industries", func(industries chi.Router) {
						industries.Get("/", srv.getAllIndustries)
						industries.Post("/", srv.createIndustry)
						industries.Put("/{industryID}", srv.updateIndustry)
						industries.Delete("/{industryID}", srv.deleteIndustry)
						industries.Get("/industries_count", srv.getIndustriesCount)
					})

					admin.Route("/groups", func(group chi.Router) {
						group.Get("/joined", srv.getAllJoinedGroupsV2)
						group.Get("/requested", srv.getAllRequestedGroupsV2)
						group.Get("/owned", srv.getGroupsCreatedByUserV2)
						group.Get("/", srv.getGroupsForAdmin)
						group.Post("/toggle_suspend", srv.suspendGroup)
						group.Post("/toggle_delete", srv.deleteGroups)
						group.Get("/export_groups", srv.exportGroups)
						group.Get("/count", srv.getGroupsCount)
						group.Get("/reported_groups", srv.getReportedGroups)
						group.Route("/{groupID}", func(group chi.Router) {
							group.Get("/details", srv.getGroupsDetails)
							group.Get("/media", srv.getAllMediaOfGroup)
							group.Get("/posts", srv.getAllPostAdmin)
							group.Get("/reported_posts", srv.getReportedPostAdmin)
							group.Get("/reported_comments", srv.getAllReportedCommentsOnPostAdmin)
							group.Get("/members", srv.getGroupMembers)
							group.Get("/membersV2", srv.getGroupMembersV2)
							group.Post("/reported_by", srv.reportedByUsersDetail)
							group.Route("/post/{postID}", func(post chi.Router) {
								post.Get("/likes", srv.getLikesOnPostByUser)
								post.Get("/comments", srv.getCommentsOnPost)
								post.Delete("/", srv.deletePostByAdmin)
								post.Route("/comment/{commentID}", func(comment chi.Router) {
									comment.Delete("/", srv.deleteCommentByAdmin)
								})
							})
						})
					})
					admin.Route("/showcase", func(showcase chi.Router) {
						showcase.Post("/update_company_status", srv.updateCompanyProfileStatus)
						showcase.Get("/all", srv.getProfilesForAdmin)
						// showcase.Get("/label_types", srv.getLabelsForShowcaseProfileAdmin)
						showcase.Get("/profiles_count", srv.getProfilesCountForAdmin)
						showcase.Get("/export", srv.exportProfiles)
						showcase.Route("/{companyID}", func(company chi.Router) {
							company.Get("/", srv.getCompanyDetailForAdmin)
							company.Put("/archive", srv.archiveCompanyProfileByAdmin)
						})
					})
					admin.Route("/jobs", func(jobs chi.Router) {
						jobs.Get("/all", srv.allJobs)
						jobs.Get("/employer", srv.employerJobsForAdmin)
						jobs.Get("/freelancer/jobs", srv.freelancerJobsForAdmin)
						jobs.Get("/freelancer/profile", srv.freelancerProfileForAdmin)
						jobs.Get("/domains", srv.allDomainsForAdmin)
						jobs.Post("/domain", srv.createJobsDomain)
						jobs.Delete("/domain", srv.deleteJobsDomain)
						jobs.Route("/domain/{domainID}", func(domain chi.Router) {
							domain.Put("/", srv.editJobsDomain)
							domain.Get("/subcategory", srv.allSubcategoryForAdmin)
							domain.Post("/subcategory", srv.createJobsSubcategory)
							domain.Delete("/subcategory", srv.deleteJobsSubcategory)
							domain.Route("/subcategory/{subcategoryID}", func(subcategory chi.Router) {
								subcategory.Put("/", srv.editJobsSubcategory)
								subcategory.Get("/skills", srv.allSkillsForAdmin)
								subcategory.Delete("/skills", srv.deleteJobsSkills)
								subcategory.Put("/skills/{skillID}", srv.editJobsSkills)
								subcategory.Post("/skills", srv.createJobsSkills)
							})
						})
						jobs.Put("/toggle_suspend", srv.jobToggleSuspend)
						jobs.Delete("/", srv.deleteJob)
						jobs.Get("/report/{jobID}", srv.getReasonsWhyReported)
						jobs.Route("/skills", func(skills chi.Router) {
							skills.Post("/", srv.createSkills)
							skills.Get("/", srv.getSkills)
							skills.Delete("/", srv.deleteSkills)
						})
						jobs.Route("/withdrawn_reasons", func(withdrawnReasons chi.Router) {
							withdrawnReasons.Post("/", srv.createWithdrawnReasons)
							withdrawnReasons.Get("/", srv.withdrawnReasons)
							withdrawnReasons.Delete("/", srv.deleteWithdrawnReasons)
						})
						jobs.Route("/{jobID}", func(job chi.Router) {
							job.Get("/applicant_count", srv.totalApplicant)
							job.Get("/proposals", srv.proposals)
							job.Get("/detail", srv.jobDetail)
						})
					})
				})
			})
			public.Route("/groups", func(groups chi.Router) {
				groups.Use(srv.Middlewares.AUTH()...)
				// groups.Use(srv.Middlewares.APITimeMiddleware()...)

				groups.Get("/list", srv.getAllUserGroups)
				groups.Get("/all", srv.getAllGroupsOfUser)
				groups.Get("/allV2", srv.getAllGroupsOfUserV2)
				groups.Get("/requested", srv.getAllRequestedGroups)
				groups.Get("/requestedV2", srv.getAllRequestedGroupsV2)
				groups.Get("/owned", srv.getGroupsCreatedByUser)
				groups.Get("/ownedV2", srv.getGroupsCreatedByUserV2)
				groups.Get("/exploreV2", srv.exploreGroupsWithPagination)
				groups.Get("/explore", srv.exploreGroups)
				groups.Get("/joined", srv.getAllJoinedGroups)
				groups.Get("/joinedV2", srv.getAllJoinedGroupsV2)
			})
			public.Route("/group", func(group chi.Router) {
				group.Use(srv.Middlewares.AUTH()...)
				// group.Use(srv.Middlewares.APITimeMiddleware()...)

				group.Get("/feeds", srv.getAllFeeds)
				group.Get("/feedsV2", srv.getAllFeedsV2)
				group.Post("/", srv.createGroup)
				group.Get("/detail/{groupID}", srv.getGroupDetail)
				group.Post("/report_type", srv.createReportType)
				group.Post("/report", srv.reportGroup)
				group.Get("/feed_group", srv.getGroupOfLatestPost)
				group.Route("/{groupID}", func(group chi.Router) {
					group.Post("/leave", srv.leaveGroup)
					group.Post("/join_request", srv.joinRequest)
					group.Post("/cancel_request", srv.cancelRequest)
					group.Get("/media_detail", srv.getAllMediaOfGroup)
					group.Get("/members", srv.getGroupMembers)
					group.Get("/membersV2", srv.getGroupMembersV2)
					group.Get("/connections", srv.getAllConnectionsForGroups)
					group.Get("/connections_v2", srv.getAllConnectionsForGroupsV2)
					group.Post("/invite_users", srv.inviteUsers)
					group.Put("/image", srv.updateGroupImage)

					group.Route("/", func(groupAdmin chi.Router) {
						groupAdmin.Use(srv.Middlewares.GroupAdminCheck()...)
						groupAdmin.Put("/", srv.updateGroup)
						groupAdmin.Delete("/", srv.deleteGroup)
						groupAdmin.Put("/toggle_admin", srv.toggleAdmin)
						groupAdmin.Post("/remove", srv.removeUserFromGroup)
						groupAdmin.Post("/decline_request", srv.declineUserGroupRequest)
						groupAdmin.Post("/accept_request", srv.acceptRequest)
						groupAdmin.Post("/block", srv.toggleBlockGroupUser)
						groupAdmin.Get("/reported_posts", srv.getReportedPost)
						groupAdmin.Get("/reported_comments", srv.getAllReportedCommentsOnPost)
						groupAdmin.Get("/blocked_users", srv.getBlockedUsers)
						groupAdmin.Get("/invites/list", srv.getInvitedUsers)
						groupAdmin.Get("/requested/list", srv.getAllRequestedUsers)
						groupAdmin.Get("/requested/list_v2", srv.getAllRequestedUsersV2)
						groupAdmin.Delete("/comment/{commentID}", srv.deleteCommentByGroupAdmin)
						groupAdmin.Delete("/post/{postID}", srv.deletePostByGroupAdmin)
					})
					group.Route("/post", func(post chi.Router) {
						post.Use(srv.Middlewares.GroupUserCheck()...)
						post.Post("/", srv.createPost)
						post.Get("/", srv.getAllPost)

						post.Route("/{postID}", func(post chi.Router) {
							post.Get("/", srv.getPostDetail)
							post.Delete("/", srv.deletePost)
							post.Put("/", srv.updatePostDetail)
							post.Post("/report", srv.reportPost)
							post.Post("/like", srv.likePost)
							post.Get("/likes", srv.getLikesOnPostByUser)
							post.Post("/comment", srv.commentOnPost)
							post.Put("/comment/{commentID}", srv.updateComment)
							post.Delete("/comment/{commentID}", srv.deleteComment)
							post.Get("/comments", srv.getCommentsOnPost)
							post.Get("/comments_v2", srv.getCommentsOnPostV2)
							post.Post("/comment/report", srv.reportComment)
							post.Get("/upvote_detail", srv.getUpVote)
							post.Route("/comments/{commentID}", func(comment chi.Router) {
								comment.Post("/like", srv.likeComment)
								comment.Post("/dislike", srv.dislikeComment)
								comment.Post("/reply", srv.commentReply)
								comment.Get("/reply", srv.getAllCommentReply)
								comment.Route("/reply/{replyID}", func(reply chi.Router) {
									reply.Delete("/", srv.deleteCommentReply)
									reply.Put("/", srv.editCommentReply)
									reply.Post("/like", srv.likeCommentReply)
									reply.Post("/dislike", srv.dislikeCommentReply)
								})
							})
						})
					})
				})
			})
			public.Route("/showcase", func(showcase chi.Router) {
				showcase.Use(srv.Middlewares.AUTH()...)
				// showcase.Use(srv.Middlewares.APITimeMiddleware()...)

				showcase.Post("/create_profile", srv.createProfile)
				showcase.Post("/bookmark", srv.toggleBookmark)
				showcase.Post("/report", srv.reportProfile) // pushNotifications
				showcase.Post("/archive", srv.archiveReplyOrQuestion)
				showcase.Get("/bookmarked_list", srv.getBookmarkedProfiles)
				showcase.Get("/bookmarked_list_v2", srv.getBookmarkedProfilesV2)
				showcase.Get("/investors", srv.getInvestorsListByName)
				showcase.Get("/archived_list", srv.getArchivedProfiles)
				showcase.Get("/profiles", srv.getCreatedProfiles)
				showcase.Get("/invested_company", srv.getAllInvestedCompany)
				// showcase.Get("/label_types", srv.getLabelsForShowcaseProfile)
				showcase.Post("/invite_users/{companyID}", srv.inviteUsersForCompany) // pushNotifications
				showcase.Get("/explore", srv.exploreCompanies)
				showcase.Get("/requests", srv.getAllInvestmentRequest)
				showcase.Put("/question", srv.DeleteQuestion)
				showcase.Get("/trending_profiles", srv.getTrendingCompanies)
				showcase.Get("/trending_profiles_v2", srv.getTrendingCompaniesV2)
				showcase.Route("/profile/{companyID}", func(profile chi.Router) {
					profile.Get("/", srv.getProfileDetail)
					profile.Post("/view", srv.increaseViewsOfCompany)
					profile.Post("/like", srv.likeCompany) // pushNotifications
					profile.Put("/", srv.editCompanyProfile)
					profile.Put("/toggle_archive", srv.toggleArchiveCompanyProfile)
					profile.Delete("/", srv.deleteCompanyProfile)
					profile.Post("/question", srv.createQuestion)        // pushNotifications
					profile.Post("/question_reply", srv.replyToQuestion) // pushNotifications
					profile.Get("/questions", srv.getQuestions)
					profile.Get("/all_questions", srv.getAllQuestions)
					profile.Get("/question_replies/{questionID}", srv.getQuestionReplies)
					profile.Get("/question_replies/{questionID}/v2", srv.getQuestionRepliesV2)
					profile.Post("/invest", srv.investInCompanyNotification) // pushNotifications
					profile.Get("/all_investors", srv.allInvestors)
					profile.Get("/all_investorsV2", srv.allInvestorsV2)
					profile.Put("/investment_status", srv.approveAndDeclineInvestment)
					profile.Get("/investment_detail", srv.getInvestmentDetail)
				})
			})

			public.Route("/dnr", srv.DnrHandler.Serve)
			public.Route("/jobs", srv.JobsHandler.Serve)
		})
	})
	return r
}
