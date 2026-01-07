package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func main() {
	fmt.Println("🧪 Running TAS Agent Builder Reliability Test Suite")
	fmt.Println("===================================================")
	fmt.Println()

	// Check for required environment variables
	requiredEnvVars := []string{"JWT_SECRET", "DB_PASSWORD"}
	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			log.Fatalf("Required environment variable %s is not set", envVar)
		}
	}

	// List of test files to run
	testFiles := []struct {
		name string
		file string
		description string
		category string
	}{
		{
			name: "Unit Tests - Reliability Models",
			file: "./test/reliability_test.go",
			description: "Tests retry/fallback config validation and model structures",
			category: "unit",
		},
		{
			name: "Integration Tests - Router Service",
			file: "./test/router_service_reliability_test.go", 
			description: "Tests enhanced router service with retry/fallback features",
			category: "integration",
		},
		{
			name: "Integration Tests - API Handlers",
			file: "./test/agent_handlers_reliability_test.go",
			description: "Tests API endpoints for reliability configuration and metrics",
			category: "integration",
		},
		{
			name: "End-to-End Integration Tests",
			file: "./test/reliability_integration_test.go",
			description: "Comprehensive database and feature integration tests",
			category: "integration",
		},
		{
			name: "Agent Lifecycle Tests",
			file: "./test/agent_lifecycle_test.go",
			description: "Complete agent creation, management, and deletion workflows",
			category: "workflow",
		},
		{
			name: "Execution Engine Tests",
			file: "./test/execution_engine_test.go",
			description: "Comprehensive execution testing with various configurations",
			category: "execution",
		},
		{
			name: "Space Management Tests",
			file: "./test/space_management_test.go",
			description: "Multi-tenant isolation and space-based access control",
			category: "security",
		},
		{
			name: "Performance Load Tests",
			file: "./test/performance_load_test.go",
			description: "Scalability validation and performance benchmarking",
			category: "performance",
		},
		{
			name: "End-to-End Integration Tests",
			file: "./test/end_to_end_integration_test.go",
			description: "Complete workflow validation and production readiness",
			category: "e2e",
		},
	}

	var failedTests []string
	totalTests := len(testFiles)
	passedTests := 0

	for i, test := range testFiles {
		fmt.Printf("📋 Test %d/%d: %s\n", i+1, totalTests, test.name)
		fmt.Printf("   Category: %s\n", test.category)
		fmt.Printf("   Description: %s\n", test.description)
		fmt.Printf("   File: %s\n", test.file)
		
		// Run the test
		cmd := exec.Command("go", "test", "-v", test.file)
		cmd.Env = os.Environ()
		
		output, err := cmd.CombinedOutput()
		
		if err != nil {
			fmt.Printf("   ❌ FAILED\n")
			fmt.Printf("   Error: %v\n", err)
			fmt.Printf("   Output: %s\n", string(output))
			failedTests = append(failedTests, test.name)
		} else {
			fmt.Printf("   ✅ PASSED\n")
			passedTests++
		}
		fmt.Println()
	}

	// Summary
	fmt.Println("📊 Test Suite Summary")
	fmt.Println("=====================")
	fmt.Printf("Total Tests: %d\n", totalTests)
	fmt.Printf("Passed: %d\n", passedTests)
	fmt.Printf("Failed: %d\n", len(failedTests))
	
	if len(failedTests) > 0 {
		fmt.Println("\n❌ Failed Tests:")
		for _, test := range failedTests {
			fmt.Printf("   - %s\n", test)
		}
		fmt.Println("\n🔍 Run individual tests with more verbose output:")
		fmt.Println("   go test -v ./test/[test_file_name]")
		os.Exit(1)
	} else {
		fmt.Println("\n🎉 All reliability tests passed!")
		fmt.Println("The enhanced reliability features are working correctly.")
		
		// Run the framework validation demo
		fmt.Println("\n🚀 Running reliability framework validation...")
		cmd := exec.Command("go", "run", "examples/test_reliability_framework.go")
		cmd.Env = os.Environ()
		output, err := cmd.CombinedOutput()
		
		if err != nil {
			fmt.Printf("⚠️  Framework validation failed: %v\n", err)
			fmt.Printf("Output: %s\n", string(output))
		} else {
			fmt.Println("✅ Framework validation completed successfully")
			
			// Show a sample of the output
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "✅") || strings.Contains(line, "All Reliability Framework Tests Completed") {
					fmt.Printf("   %s\n", line)
				}
			}
		}
	}
	
	fmt.Println("\n🏁 Test suite execution complete!")
}