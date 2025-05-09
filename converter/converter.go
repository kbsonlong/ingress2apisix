package converter

import (
	"fmt"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApisixRoute 表示 APISIX Route 资源
type ApisixRoute struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec              ApisixRouteSpec `json:"spec" yaml:"spec"`
}

// ApisixRouteSpec 定义 APISIX Route 的规格
type ApisixRouteSpec struct {
	IngressClassName *string           `json:"ingressClassName,omitempty" yaml:"ingressClassName,omitempty"`
	HTTP             []ApisixHTTPRoute `json:"http" yaml:"http"`
}

// ApisixHTTPRoute 定义 APISIX HTTP 路由
type ApisixHTTPRoute struct {
	Name     string          `json:"name" yaml:"name"`
	Match    ApisixMatch     `json:"match" yaml:"match"`
	Backends []ApisixBackend `json:"backends" yaml:"backends"`
	Plugins  []ApisixPlugin  `json:"plugins,omitempty" yaml:"plugins,omitempty"`
}

// ApisixMatch 定义 APISIX 路由匹配规则
type ApisixMatch struct {
	Hosts  []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
	Paths  []string `json:"paths" yaml:"paths"`
	Method []string `json:"method,omitempty" yaml:"method,omitempty"`
}

// ApisixBackend 定义 APISIX 后端服务
type ApisixBackend struct {
	ServiceName string `json:"serviceName" yaml:"serviceName"`
	ServicePort int32  `json:"servicePort" yaml:"servicePort"`
	Weight      int    `json:"weight,omitempty" yaml:"weight,omitempty"`
}

// ApisixPlugin 定义 APISIX 插件
type ApisixPlugin struct {
	Name   string                 `json:"name" yaml:"name"`
	Enable bool                   `json:"enable" yaml:"enable"`
	Config map[string]interface{} `json:"config" yaml:"config"`
}

// ApisixTLS 定义 APISIX TLS 配置
type ApisixTLS struct {
	Hosts      []string `json:"hosts"`
	SecretName string   `json:"secretName"`
}

// 常见的 Ingress 注解键
const (
	// 重写路径注解
	AnnotationRewriteTarget = "nginx.ingress.kubernetes.io/rewrite-target"
	// SSL 重定向注解
	AnnotationSSLRedirect = "nginx.ingress.kubernetes.io/ssl-redirect"
	// 强制 SSL 重定向注解
	AnnotationForceSSLRedirect = "nginx.ingress.kubernetes.io/force-ssl-redirect"
	// 启用 CORS 注解
	AnnotationEnableCORS = "nginx.ingress.kubernetes.io/enable-cors"
	// CORS 允许方法注解
	AnnotationCorsAllowMethods = "nginx.ingress.kubernetes.io/cors-allow-methods"
	// CORS 允许源注解
	AnnotationCorsAllowOrigin = "nginx.ingress.kubernetes.io/cors-allow-origin"
	// 启用限流注解
	AnnotationEnableRateLimit = "nginx.ingress.kubernetes.io/limit-rps"
)

// convertPath 将 Ingress 路径转换为 APISIX 路径
func convertPath(path string) string {
	// 如果路径以 /* 结尾，直接返回
	if strings.HasSuffix(path, "/*") {
		return path
	}

	// 如果路径包含正则表达式，转换为通配符格式
	if strings.Contains(path, "(/|$)") {
		// 移除 (/|$) 部分
		path = strings.Replace(path, "(/|$)", "", -1)
		// 移除 (.*) 部分
		path = strings.Replace(path, "(.*)", "", -1)
		// 添加通配符
		path = path + "/*"
	}

	return path
}

// ConvertToApisixRoute 将 Kubernetes Ingress 转换为 APISIX Route
func ConvertToApisixRoute(ingress networkingv1.Ingress) (*ApisixRoute, error) {
	if len(ingress.Spec.Rules) == 0 {
		return nil, fmt.Errorf("ingress has no rules")
	}

	route := &ApisixRoute{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ApisixRoute",
			APIVersion: "apisix.apache.org/v2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingress.Name,
			Namespace: ingress.Namespace,
		},
		Spec: ApisixRouteSpec{
			IngressClassName: ingress.Spec.IngressClassName,
			HTTP:             make([]ApisixHTTPRoute, 0),
		},
	}

	// 处理每个规则
	for _, rule := range ingress.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			httpRoute := ApisixHTTPRoute{
				Name: fmt.Sprintf("%s-%s", ingress.Name, path.Backend.Service.Name),
				Match: ApisixMatch{
					Hosts: []string{rule.Host},
					Paths: []string{convertPath(path.Path)},
				},
				Backends: []ApisixBackend{
					{
						ServiceName: path.Backend.Service.Name,
						ServicePort: path.Backend.Service.Port.Number,
					},
				},
			}

			// 处理注解
			if ingress.Annotations != nil {
				httpRoute.Plugins = make([]ApisixPlugin, 0)
				convertAnnotations(ingress.Annotations, &httpRoute)
			}

			route.Spec.HTTP = append(route.Spec.HTTP, httpRoute)
		}
	}

	return route, nil
}

// convertAnnotations 转换 Ingress 注解到 APISIX 插件配置
func convertAnnotations(annotations map[string]string, route *ApisixHTTPRoute) {
	// 处理重写路径
	if rewriteTarget, ok := annotations[AnnotationRewriteTarget]; ok {
		route.Plugins = append(route.Plugins, ApisixPlugin{
			Name: "proxy-rewrite",
			Config: map[string]interface{}{
				"uri": rewriteTarget,
			},
			Enable: true,
		})
	}

	// 处理 SSL 重定向
	if sslRedirect, ok := annotations[AnnotationSSLRedirect]; ok && sslRedirect == "true" {
		route.Plugins = append(route.Plugins, ApisixPlugin{
			Name: "redirect",
			Config: map[string]interface{}{
				"http_to_https": true,
			},
			Enable: true,
		})
	}

	// 处理 CORS
	if enableCORS, ok := annotations[AnnotationEnableCORS]; ok && enableCORS == "true" {
		corsConfig := map[string]interface{}{
			"allow_origins":  "*",
			"allow_methods":  "GET,POST,PUT,DELETE,OPTIONS",
			"allow_headers":  "*",
			"expose_headers": "*",
			"max_age":        3600,
		}

		// 处理 CORS 允许方法
		if methods, ok := annotations[AnnotationCorsAllowMethods]; ok {
			corsConfig["allow_methods"] = methods
		}

		// 处理 CORS 允许源
		if origins, ok := annotations[AnnotationCorsAllowOrigin]; ok {
			corsConfig["allow_origins"] = origins
		}

		route.Plugins = append(route.Plugins, ApisixPlugin{
			Name:   "cors",
			Config: corsConfig,
			Enable: true,
		})
	}

	// 处理限流
	if limitRPS, ok := annotations[AnnotationEnableRateLimit]; ok {
		route.Plugins = append(route.Plugins, ApisixPlugin{
			Name: "limit-req",
			Config: map[string]interface{}{
				"rate":          limitRPS,
				"burst":         0,
				"rejected_code": 503,
			},
			Enable: true,
		})
	}

	// 处理自定义注解
	for key, value := range annotations {
		if strings.HasPrefix(key, "apisix.ingress.kubernetes.io/") {
			pluginName := strings.TrimPrefix(key, "apisix.ingress.kubernetes.io/")
			// 尝试解析 JSON 格式的插件配置
			if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
				route.Plugins = append(route.Plugins, ApisixPlugin{
					Name:   pluginName,
					Config: map[string]interface{}{"value": value},
					Enable: true,
				})
			} else {
				route.Plugins = append(route.Plugins, ApisixPlugin{
					Name:   pluginName,
					Config: map[string]interface{}{"value": value},
					Enable: true,
				})
			}
		}
	}
}
