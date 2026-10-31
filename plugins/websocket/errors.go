package websocket

import apperrors "github.com/alldev-run/golang-gin-rpc/pkg/errors"

func IsProtobufMessageNilError(err error) bool {
	return apperrors.IsCode(err, apperrors.ErrCodeWebSocketProtoMessageNil)
}

func IsProtobufDestinationNilError(err error) bool {
	return apperrors.IsCode(err, apperrors.ErrCodeWebSocketProtoDestinationNil)
}

func IsProtobufPayloadTypeMismatchError(err error) bool {
	return apperrors.IsCode(err, apperrors.ErrCodeWebSocketProtoPayloadType)
}

func IsProtobufDestinationTypeMismatchError(err error) bool {
	return apperrors.IsCode(err, apperrors.ErrCodeWebSocketProtoDestinationType)
}

func IsProtobufFrameTypeMismatchError(err error) bool {
	return apperrors.IsCode(err, apperrors.ErrCodeWebSocketProtoFrameType)
}
