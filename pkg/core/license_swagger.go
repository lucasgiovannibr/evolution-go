package core

import "github.com/gin-gonic/gin"

// The license routes are registered by LicenseRoutes with inline handlers, which
// swag cannot annotate. These declarations exist only so that `make swagger` keeps
// documenting them; nothing calls them.

var _ gin.H // the annotations below name gin.H

// @Summary Get license status
// @Description Returns whether the instance license is active, along with the instance id and a masked api key.
// @Tags License
// @Produce json
// @Success 200 {object} gin.H "License status ({status, instance_id, api_key?})"
// @Router /license/status [get]
func swaggerLicenseStatus() {}

// @Summary Register / get registration URL
// @Description Checks the GLOBAL_API_KEY with the licensing server. If not yet registered, initiates registration and returns a register_url. Accepts an optional redirect_uri for the post-registration redirect.
// @Tags License
// @Produce json
// @Param redirect_uri query string false "Post-registration redirect URI"
// @Success 200 {object} gin.H "Registration state (status/message or register_url)"
// @Router /license/register [get]
func swaggerLicenseRegister() {}

// @Summary Activate license
// @Description Exchanges an authorization code (from the registration callback) for an api_key and persists it. Provide the code via the query string.
// @Tags License
// @Produce json
// @Param code query string true "Authorization code from the registration callback"
// @Success 200 {object} gin.H "Activation result"
// @Failure 400 {object} gin.H "Missing code parameter"
// @Router /license/activate [get]
func swaggerLicenseActivate() {}
