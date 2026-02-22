package v1

import (
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/gofiber/fiber/v2"
)

func errorResponse(ctx *fiber.Ctx, code int, msg string) error {
	requestID, _ := ctx.Locals("request_id").(string)

	return ctx.Status(code).JSON(response.NewErrorResponse(requestID, "", msg))
}

func appErrorResponse(ctx *fiber.Ctx, err error) error {
	appErr := entity.GetAppError(err)
	requestID, _ := ctx.Locals("request_id").(string)

	return ctx.Status(appErr.HTTPStatus).JSON(response.NewErrorResponse(requestID, appErr.Code, appErr.Message))
}
