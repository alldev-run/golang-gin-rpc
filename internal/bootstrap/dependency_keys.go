package bootstrap

// Dependency keys used by the bootstrap dependency container.
const (
	DepDBFactory          = "db.factory"           // *db.Factory
	DepCache              = "cache"                // cache.Cache
	DepRPCManager         = "rpc.manager"          // *rpc.Manager
	DepDiscoveryManager   = "discovery.manager"    // *discovery.ServiceDiscoveryManager
	DepErrorsInitialized  = "errors.initialized"   // bool
	DepHealthManager      = "health.manager"       // *health.HealthManager
	DepMetricsCollector   = "metrics.collector"    // *metrics.MetricsCollector
	DepAuthManager        = "auth.manager"         // *auth.AuthManager
	DepTracer             = "tracer"               // *tracing.Tracer
	DepUploader           = "upload.uploader"      // *upload.Uploader
	DepActorSystem        = "actor.system"         // *actor.System
	DepGateway            = "gateway"              // *gateway.Gateway
	DepGatewayHTTPService = "gateway.http_service" // gateway HTTP service
	DepWebSocketServer    = "websocket.server"     // *websocket.Server
	DepWebSocketManager   = "websocket.manager"    // *websocket.Manager
)
