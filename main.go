package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"ingress-converter/converter"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"gopkg.in/yaml.v3"
)

// 支持的输出格式
const (
	FormatJSON = "json"
	FormatYAML = "yaml"
)

func main() {
	// 定义命令行参数
	inputDir := flag.String("input", "", "输入目录，包含 Ingress YAML 文件")
	outputDir := flag.String("output", "output", "输出目录，用于保存转换后的 APISIX Route 文件")
	format := flag.String("format", FormatJSON, "输出格式：json 或 yaml")
	flag.Parse()

	// 验证输入目录
	if *inputDir == "" {
		fmt.Println("错误：必须指定输入目录")
		flag.Usage()
		os.Exit(1)
	}

	// 验证输出格式
	if *format != FormatJSON && *format != FormatYAML {
		fmt.Printf("错误：不支持的输出格式 %s，支持的格式：%s, %s\n", *format, FormatJSON, FormatYAML)
		os.Exit(1)
	}

	// 创建输出目录
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Printf("创建输出目录失败：%v\n", err)
		os.Exit(1)
	}

	// 创建 Kubernetes 解码器
	scheme := runtime.NewScheme()
	if err := networkingv1.AddToScheme(scheme); err != nil {
		fmt.Printf("创建 Scheme 失败：%v\n", err)
		os.Exit(1)
	}
	codecs := serializer.NewCodecFactory(scheme)
	decoder := codecs.UniversalDeserializer()

	// 读取输入目录中的所有 YAML 文件
	files, err := ioutil.ReadDir(*inputDir)
	if err != nil {
		fmt.Printf("读取输入目录失败：%v\n", err)
		os.Exit(1)
	}

	// 处理每个 YAML 文件
	for _, file := range files {
		if !isYAMLFile(file.Name()) {
			continue
		}

		fmt.Printf("处理文件：%s\n", file.Name())

		// 读取文件内容
		content, err := ioutil.ReadFile(filepath.Join(*inputDir, file.Name()))
		if err != nil {
			fmt.Printf("读取文件 %s 失败：%v\n", file.Name(), err)
			continue
		}

		// 分割 YAML 内容（处理多文档 YAML）
		docs := splitYAMLDocuments(string(content))
		fmt.Printf("找到 %d 个 YAML 文档\n", len(docs))

		// 处理每个 YAML 文档
		for i, doc := range docs {
			if strings.TrimSpace(doc) == "" {
				continue
			}

			fmt.Printf("处理第 %d 个文档：\n%s\n", i+1, doc)

			// 解析 Ingress 资源
			obj, _, err := decoder.Decode([]byte(doc), nil, nil)
			if err != nil {
				fmt.Printf("解析 YAML 文档失败：%v\n", err)
				continue
			}

			ingress, ok := obj.(*networkingv1.Ingress)
			if !ok {
				fmt.Printf("文档不是 Ingress 资源\n")
				continue
			}

			fmt.Printf("解析结果：Kind=%s, Name=%s\n", ingress.Kind, ingress.Name)

			// 转换为 APISIX Route
			route, err := converter.ConvertToApisixRoute(*ingress)
			if err != nil {
				fmt.Printf("转换 Ingress %s 失败：%v\n", ingress.Name, err)
				continue
			}

			// 生成输出文件名
			outputFile := filepath.Join(*outputDir, fmt.Sprintf("%s-%s.%s", ingress.Namespace, ingress.Name, *format))

			// 写入输出文件
			if err := writeToFile(route, outputFile, *format); err != nil {
				fmt.Printf("写入文件 %s 失败：%v\n", outputFile, err)
				continue
			}

			fmt.Printf("已转换 Ingress %s（命名空间：%s）到 %s\n", ingress.Name, ingress.Namespace, outputFile)
		}
	}
}

// isYAMLFile 检查文件是否为 YAML 文件
func isYAMLFile(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".yaml") || strings.HasSuffix(strings.ToLower(filename), ".yml")
}

// splitYAMLDocuments 分割多文档 YAML 内容
func splitYAMLDocuments(content string) []string {
	return strings.Split(content, "---")
}

// writeToFile 将数据写入文件
func writeToFile(data interface{}, filename string, format string) error {
	var content []byte
	var err error

	if format == FormatJSON {
		content, err = json.MarshalIndent(data, "", "  ")
	} else {
		// 先转换为 JSON 以去除空字段
		jsonData, err := json.Marshal(data)
		if err != nil {
			return err
		}

		// 创建一个临时 map 来存储数据
		var tempMap map[string]interface{}
		if err := json.Unmarshal(jsonData, &tempMap); err != nil {
			return err
		}

		// 修复 apiVersion 字段
		if apiVersion, ok := tempMap["apiversion"]; ok {
			tempMap["apiVersion"] = apiVersion
			delete(tempMap, "apiversion")
		}

		// 创建 YAML 编码器
		content, err = yaml.Marshal(tempMap)
	}

	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, content, 0644)
}
