package log

import (
	"sync"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/log"
)

type HandlerCreatorOptions struct {
	Path string
}

type HandlerCreator func(LogType, HandlerCreatorOptions) (log.Handler, error)

var handlerCreatorMap = make(map[LogType]HandlerCreator)

var handlerCreatorMapLock = &sync.RWMutex{}

func RegisterHandlerCreator(logType LogType, f HandlerCreator) error {
	if f == nil {
		return errors.New("nil HandlerCreator")
	}

	handlerCreatorMapLock.Lock()
	defer handlerCreatorMapLock.Unlock()

	handlerCreatorMap[logType] = f
	return nil
}

func createHandler(logType LogType, options HandlerCreatorOptions) (log.Handler, error) {
	handlerCreatorMapLock.RLock()
	defer handlerCreatorMapLock.RUnlock()

	creator, found := handlerCreatorMap[logType]
	if !found {
		return nil, errors.New("unable to create log handler for ", logType)
	}
	return creator(logType, options)
}

func init() {
	common.Must(RegisterHandlerCreator(LogType_Console, func(lt LogType, options HandlerCreatorOptions) (log.Handler, error) {
		return log.NewLogger(log.CreateStdoutLogWriter()), nil
	}))

	common.Must(RegisterHandlerCreator(LogType_File, func(lt LogType, options HandlerCreatorOptions) (log.Handler, error) {
		creator, err := log.CreateFileLogWriter(options.Path)
		if err != nil {
			return nil, err
		}
		return log.NewLogger(creator), nil
	}))

	common.Must(RegisterHandlerCreator(LogType_None, func(lt LogType, options HandlerCreatorOptions) (log.Handler, error) {
		return nil, nil
	}))
}
