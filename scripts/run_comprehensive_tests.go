package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	fmt.Println("🧪 TAS Agent Builder - Comprehensive Test Suite")
	fmt.Println("===============================================")
	fmt.Println()

	// Command line flags
	var (
		category = flag.String("category", "all", "Test category to run (unit, integration, workflow, execution, security, performance, e2e, all)")
		verbose  = flag.Bool("verbose", false, "Enable verbose output")
		short    = flag.Bool("short", false, "Run tests in short mode (skip long-running tests)")
	)
	flag.Parse()

	// Check for required environment variables
	requiredEnvVars := []string{"JWT_SECRET", "DB_PASSWORD"}
	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			log.Fatalf("Required environment variable %s is not set", envVar)
		}
	}

	// Define all test suites
	testSuites := map[string][]TestFile{
		"unit": {
			{
				Name:        "Reliability Models Unit Tests",
				File:        "./test/reliability_test.go",
				Description: "Tests retry/fallback config validation and model structures",
				Duration:    "~30s",
			},
		},
		"integration": {
			{
				Name:        "Router Service Integration",
				File:        "./test/router_service_reliability_test.go",
				Description: "Enhanced router service with retry/fallback features",
				Duration:    "~1m",
			},
			{
				Name:        "API Handlers Integration",
				File:        "./test/agent_handlers_reliability_test.go",
				Description: "API endpoints for reliability configuration and metrics",
				Duration:    "~45s",
			},
			{
				Name:        "Database Integration",
				File:        "./test/reliability_integration_test.go",
				Description: "Database schema, views, and analytics functions",
				Duration:    "~1m",
			},
			{
				Name:        "Provider Validation",
				File:        "./test/provider_validation_test.go",
				Description: "Multi-provider integration (OpenAI, Anthropic)",
				Duration:    "~2m",
			},
			{
				Name:        "Router Integration",
				File:        "./test/router_integration_test.go",
				Description: "Basic router connectivity and provider routing",
				Duration:    "~1m",
			},
		},
		"workflow": {
			{
				Name:        "Agent Lifecycle Tests",
				File:        "./test/agent_lifecycle_test.go",
				Description: "Complete agent creation, management, and deletion workflows",
				Duration:    "~2m",
			},
		},
		"execution": {
			{
				Name:        "Execution Engine Tests",
				File:        "./test/execution_engine_test.go",
				Description: "Comprehensive execution testing with various configurations",
				Duration:    "~3m",
			},
		},
		"security": {
			{
				Name:        "Space Management Tests",
				File:        "./test/space_management_test.go",
				Description: "Multi-tenant isolation and space-based access control",
				Duration:    "~1m",
			},
		},
		"performance": {
			{
				Name:        "Performance Load Tests",
				File:        "./test/performance_load_test.go",
				Description: "Scalability validation and performance benchmarking",
				Duration:    "~5m",
				LongRunning: true,
			},
		},
		"e2e": {
			{
				Name:        "End-to-End Integration",
				File:        "./test/end_to_end_integration_test.go",
				Description: "Complete workflow validation and production readiness",
				Duration:    "~3m",
			},
		},
	}

	// Determine which test suites to run
	var suitesToRun []string
	if *category == "all" {
		for suite := range testSuites {
			suitesToRun = append(suitesToRun, suite)
		}
	} else {
		if _, exists := testSuites[*category]; !exists {
			log.Fatalf("Unknown test category: %s. Available: %s", *category, strings.Join(getCategories(testSuites), ", "))
		}
		suitesToRun = []string{*category}
	}

	// Run test suites
	var results []SuiteResult
	totalStart := time.Now()

	for _, suiteCategory := range suitesToRun {
		suite := testSuites[suiteCategory]
		fmt.Printf("🏃 Running %s Tests (%d tests)\n", strings.Title(suiteCategory), len(suite))
		fmt.Println(strings.Repeat("=", 50))

		suiteResult := runTestSuite(suiteCategory, suite, *verbose, *short)
		results = append(results, suiteResult)

		fmt.Printf("✅ %s tests completed: %d/%d passed (%.1f%%)\n\n", 
			strings.Title(suiteCategory), suiteResult.PassedCount, suiteResult.TotalCount, suiteResult.SuccessRate)
	}

	totalDuration := time.Since(totalStart)

	// Print comprehensive summary
	printTestSummary(results, totalDuration)

	// Exit with appropriate code
	allPassed := true
	for _, result := range results {
		if result.FailedCount > 0 {
			allPassed = false
			break
		}
	}

	if allPassed {
		fmt.Println("🎉 All tests passed! The system is ready for production.")
		os.Exit(0)
	} else {
		fmt.Println("❌ Some tests failed. Please review the results above.")
		os.Exit(1)
	}
}

type TestFile struct {
	Name        string
	File        string
	Description string
	Duration    string
	LongRunning bool
}

type SuiteResult struct {
	Category     string
	TotalCount   int
	PassedCount  int
	FailedCount  int
	SuccessRate  float64
	Duration     time.Duration
	FailedTests  []string
}

func runTestSuite(category string, tests []TestFile, verbose, short bool) SuiteResult {
	suiteStart := time.Now()
	var passedCount, failedCount int
	var failedTests []string

	for i, test := range tests {
		// Skip long-running tests in short mode
		if short && test.LongRunning {
			fmt.Printf("⏭️  Test %d/%d: %s (SKIPPED - long running)\n", i+1, len(tests), test.Name)
			continue
		}

		fmt.Printf("📋 Test %d/%d: %s\n", i+1, len(tests), test.Name)
		fmt.Printf("   Description: %s\n", test.Description)
		fmt.Printf("   Expected duration: %s\n", test.Duration)
		fmt.Printf("   File: %s\n", test.File)

		// Prepare test command
		args := []string{"test", "-v", test.File}
		if short {
			args = append(args, "-short")
		}

		cmd := exec.Command("go", args...)
		cmd.Env = os.Environ()

		testStart := time.Now()
		output, err := cmd.CombinedOutput()
		testDuration := time.Since(testStart)

		if err != nil {
			fmt.Printf("   ❌ FAILED (%s)\n", testDuration.Round(time.Second))
			failedCount++
			failedTests = append(failedTests, test.Name)
			
			if verbose {
				fmt.Printf("   Error: %v\n", err)
				fmt.Printf("   Output:\n%s\n", indentOutput(string(output)))
			}
		} else {
			fmt.Printf("   ✅ PASSED (%s)\n", testDuration.Round(time.Second))
			passedCount++
			
			if verbose {
				fmt.Printf("   Output:\n%s\n", indentOutput(string(output)))
			}
		}
		fmt.Println()
	}

	suiteDuration := time.Since(suiteStart)
	totalCount := passedCount + failedCount
	successRate := float64(passedCount) / float64(totalCount) * 100

	return SuiteResult{
		Category:    category,
		TotalCount:  totalCount,
		PassedCount: passedCount,
		FailedCount: failedCount,
		SuccessRate: successRate,
		Duration:    suiteDuration,
		FailedTests: failedTests,
	}
}

func printTestSummary(results []SuiteResult, totalDuration time.Duration) {
	fmt.Println("📊 Comprehensive Test Results Summary")
	fmt.Println("====================================")

	var grandTotalTests, grandTotalPassed, grandTotalFailed int
	
	for _, result := range results {
		status := "✅"
		if result.FailedCount > 0 {
			status = "❌"
		}

		fmt.Printf("%s %-12s: %d/%d passed (%.1f%%) - %s\n",
			status, strings.Title(result.Category), result.PassedCount, result.TotalCount, 
			result.SuccessRate, result.Duration.Round(time.Second))

		grandTotalTests += result.TotalCount
		grandTotalPassed += result.PassedCount
		grandTotalFailed += result.FailedCount
	}

	fmt.Println(strings.Repeat("-", 50))
	grandSuccessRate := float64(grandTotalPassed) / float64(grandTotalTests) * 100
	fmt.Printf("📈 Overall Results: %d/%d passed (%.1f%%) - %s\n",
		grandTotalPassed, grandTotalTests, grandSuccessRate, totalDuration.Round(time.Second))

	// Show failed tests if any
	if grandTotalFailed > 0 {
		fmt.Println("\n❌ Failed Tests:")
		for _, result := range results {
			if len(result.FailedTests) > 0 {
				fmt.Printf("   %s category:\n", strings.Title(result.Category))
				for _, failedTest := range result.FailedTests {
					fmt.Printf("     - %s\n", failedTest)
				}
			}
		}
		
		fmt.Println("\n🔍 To run individual tests:")
		fmt.Println("   go test -v ./test/[test_file_name]")
		fmt.Println("   or use: go run scripts/run_comprehensive_tests.go -category [category]")
	}

	// Performance analysis
	fmt.Println("\n⚡ Performance Analysis:")
	fmt.Printf("   Total execution time: %s\n", totalDuration.Round(time.Second))
	fmt.Printf("   Average time per test: %s\n", (totalDuration / time.Duration(grandTotalTests)).Round(time.Second))
	
	if grandSuccessRate >= 95.0 {
		fmt.Println("   Status: 🌟 Excellent - Ready for production")
	} else if grandSuccessRate >= 90.0 {
		fmt.Println("   Status: ✅ Good - Minor issues to address")
	} else if grandSuccessRate >= 80.0 {
		fmt.Println("   Status: ⚠️  Fair - Several issues need attention")
	} else {
		fmt.Println("   Status: ❌ Poor - Significant issues require resolution")
	}
}

func getCategories(testSuites map[string][]TestFile) []string {
	var categories []string
	for category := range testSuites {
		categories = append(categories, category)
	}
	return categories
}

func indentOutput(output string) string {
	lines := strings.Split(output, "\n")
	var indentedLines []string
	for _, line := range lines {
		if line != "" {
			indentedLines = append(indentedLines, "     "+line)
		}
	}
	return strings.Join(indentedLines, "\n")
}