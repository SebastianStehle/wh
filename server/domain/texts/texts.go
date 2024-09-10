package texts

import (
	"context"
	"wh/infrastructure/server"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func getTextCore(c context.Context, config *i18n.LocalizeConfig) string {
	localizer := server.GetLocalizer(c)
	if localizer == nil {
		return ""
	}

	text, _ := localizer.Localize(config)
	return text
}

func getText(c context.Context, id string, other string) string {
	return getTextCore(c, &i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    id,
			Other: other,
		},
	})
}

func CommonError(c context.Context) string {
	return getText(c, "common.error", "An error has been occurred. We are probably already working on that and will provide a fix as soon as possible.")
}

func CommonErrorNotFound(c context.Context) string {
	return getText(c, "common.errorNotFound", "The page your are looking for does not exist. Perhaps the resource has been deleted by another user.")
}

func CommonRequest(c context.Context) string {
	return getText(c, "common.request", "Request")
}

func CommonResponse(c context.Context) string {
	return getText(c, "common.response", "Response")
}

func CommonHeader(c context.Context) string {
	return getText(c, "common.header", "Header")
}

func CommonValue(c context.Context) string {
	return getText(c, "common.value", "Value")
}

func CommonWelcomeTitle(c context.Context) string {
	return getText(c, "common.welcomeTitle", "Hello Developer 😀")
}

func CommonRequests(c context.Context) string {
	return getText(c, "common.commonRequests", "HTTP Requests")
}

func CommonRequestsEmpty(c context.Context) string {
	return getText(c, "common.requestsEmpty", "No Requests recorded yet")
}

func CommonWelcomeLogin(c context.Context) string {
	return getText(c, "common.welcomeLogin", "Please enter the API Key to continue. The key is defined in the configuration. Please ask your admin if you have not installed this container.")
}

func CommonApiKey(c context.Context) string {
	return getText(c, "common.apiKey", "API Key")
}

func CommonContinue(c context.Context) string {
	return getText(c, "common.continue", "Continue")
}

func CommonInvalidApiKey(c context.Context) string {
	return getText(c, "common.invalidApiKey", "Invalid API Key")
}

func CommonRequestTimeoutText(c context.Context) string {
	return getText(c, "common.requestTimeout", "Request has not been answered in time.")
}

func CommonRequestTimeoutLabel(c context.Context) string {
	return getText(c, "common.requestTimeout", "Timeout")
}

func CommonRequestErrorText(c context.Context) string {
	return getText(c, "common.requestError", "Internal error to handle the request. Please check the logs.")
}

func CommonRequestErrorLabel(c context.Context) string {
	return getText(c, "common.requestError", "Error")
}

func CommonBodyNotRendered(c context.Context) string {
	return getText(c, "common.bodyNotRendered", "Body cannot be rendered")
}

func CommonDuration(c context.Context) string {
	return getText(c, "common.duration", "Duration")
}
