#!/usr/bin/env python3
"""
TickHook Performance Benchmark Script
Runs Go benchmarks, API load tests, and generates performance graphs.
"""

import subprocess
import json
import re
import os
import sys
from datetime import datetime

# Check for matplotlib
try:
    import matplotlib
    matplotlib.use('Agg')  # Non-interactive backend
    import matplotlib.pyplot as plt
    import matplotlib.patches as mpatches
except ImportError:
    print("matplotlib not installed, skipping graph generation")
    plt = None

# Paths
PROJECT_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DOCS_IMG_DIR = os.path.join(PROJECT_ROOT, "docs", "img")
BENCH_RESULTS_FILE = os.path.join(PROJECT_ROOT, "benchmark_results.json")

def ensure_dirs():
    """Ensure output directories exist."""
    os.makedirs(DOCS_IMG_DIR, exist_ok=True)

def run_go_benchmarks():
    """Run Go benchmarks and parse results."""
    print("Running Go benchmarks...")

    results = {}

    # Run util benchmarks
    cmd = ["go", "test", "-bench=.", "-benchmem", "-count=5", "./internal/util/"]
    proc = subprocess.run(cmd, capture_output=True, text=True, cwd=PROJECT_ROOT)
    results["util"] = parse_bench_output(proc.stdout)

    # Run store benchmarks (requires Redis)
    cmd = ["go", "test", "-bench=.", "-benchmem", "-count=3", "./internal/store/"]
    proc = subprocess.run(cmd, capture_output=True, text=True, cwd=PROJECT_ROOT)
    results["store"] = parse_bench_output(proc.stdout)

    return results

def parse_bench_output(output):
    """Parse Go benchmark output."""
    benchmarks = {}

    # Pattern: BenchmarkName-N    iterations    ns/op    bytes/op    allocs/op
    pattern = r'(Benchmark\w+)-\d+\s+(\d+)\s+([\d.]+)\s+ns/op(?:\s+([\d.]+)\s+B/op)?(?:\s+(\d+)\s+allocs/op)?'

    for match in re.finditer(pattern, output):
        name = match.group(1)
        iterations = int(match.group(2))
        ns_per_op = float(match.group(3))
        bytes_per_op = float(match.group(4)) if match.group(4) else 0
        allocs_per_op = int(match.group(5)) if match.group(5) else 0

        if name not in benchmarks:
            benchmarks[name] = {
                "iterations": [],
                "ns_per_op": [],
                "bytes_per_op": [],
                "allocs_per_op": []
            }

        benchmarks[name]["iterations"].append(iterations)
        benchmarks[name]["ns_per_op"].append(ns_per_op)
        benchmarks[name]["bytes_per_op"].append(bytes_per_op)
        benchmarks[name]["allocs_per_op"].append(allocs_per_op)

    # Average the results
    for name, data in benchmarks.items():
        benchmarks[name] = {
            "iterations": sum(data["iterations"]) // len(data["iterations"]),
            "ns_per_op": sum(data["ns_per_op"]) / len(data["ns_per_op"]),
            "us_per_op": sum(data["ns_per_op"]) / len(data["ns_per_op"]) / 1000,
            "bytes_per_op": sum(data["bytes_per_op"]) / len(data["bytes_per_op"]),
            "allocs_per_op": sum(data["allocs_per_op"]) // len(data["allocs_per_op"]),
            "ops_per_sec": 1_000_000_000 / (sum(data["ns_per_op"]) / len(data["ns_per_op"]))
        }

    return benchmarks

def measure_binary_size():
    """Measure the compiled binary size."""
    print("Measuring binary size...")

    # Build the binary
    cmd = ["go", "build", "-ldflags=-w -s", "-o", "tickhook-bench", "./cmd/tickhook"]
    subprocess.run(cmd, cwd=PROJECT_ROOT, check=True)

    binary_path = os.path.join(PROJECT_ROOT, "tickhook-bench")
    size_bytes = os.path.getsize(binary_path)

    # Clean up
    os.remove(binary_path)

    return {
        "bytes": size_bytes,
        "kb": size_bytes / 1024,
        "mb": size_bytes / 1024 / 1024
    }

def measure_memory_usage():
    """Measure memory usage at startup."""
    print("Measuring memory footprint...")

    # Build the binary
    cmd = ["go", "build", "-o", "tickhook-bench", "./cmd/tickhook"]
    subprocess.run(cmd, cwd=PROJECT_ROOT, check=True)

    binary_path = os.path.join(PROJECT_ROOT, "tickhook-bench")

    # Start the process and measure memory
    import time

    proc = subprocess.Popen(
        [binary_path, "--redis-url", "redis://localhost:6379", "--auth-token", "bench-token"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        cwd=PROJECT_ROOT
    )

    # Wait for startup
    time.sleep(2)

    # Get memory usage
    try:
        with open(f"/proc/{proc.pid}/status", "r") as f:
            status = f.read()

        rss = 0
        vms = 0
        for line in status.split("\n"):
            if line.startswith("VmRSS:"):
                rss = int(line.split()[1])  # KB
            elif line.startswith("VmSize:"):
                vms = int(line.split()[1])  # KB
    except:
        rss = 0
        vms = 0

    proc.terminate()
    proc.wait()

    # Clean up
    os.remove(binary_path)

    return {
        "rss_kb": rss,
        "rss_mb": rss / 1024,
        "vms_kb": vms,
        "vms_mb": vms / 1024
    }

def run_api_benchmark():
    """Run HTTP API benchmarks using wrk or curl."""
    print("Running API benchmarks...")

    import time

    # Build and start the server
    cmd = ["go", "build", "-o", "tickhook-bench", "./cmd/tickhook"]
    subprocess.run(cmd, cwd=PROJECT_ROOT, check=True)

    binary_path = os.path.join(PROJECT_ROOT, "tickhook-bench")

    proc = subprocess.Popen(
        [binary_path, "--redis-url", "redis://localhost:6379", "--auth-token", "bench-token", "--bind", "127.0.0.1:18080"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        cwd=PROJECT_ROOT
    )

    time.sleep(2)  # Wait for startup

    results = {}

    try:
        # Test health endpoint with wrk
        cmd = ["wrk", "-t4", "-c100", "-d10s", "http://127.0.0.1:18080/health"]
        wrk_result = subprocess.run(cmd, capture_output=True, text=True)
        results["health"] = parse_wrk_output(wrk_result.stdout)

        # Test job creation (need to use a Lua script for POST)
        # For simplicity, use curl loop
        start = time.time()
        success_count = 0
        for i in range(1000):
            curl_cmd = [
                "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
                "-X", "POST",
                "-H", "Authorization: Bearer bench-token",
                "-H", "Content-Type: application/json",
                "-d", json.dumps({
                    "execute_at": "2030-01-01T00:00:00Z",
                    "webhook": {
                        "url": "https://example.com/hook",
                        "method": "POST"
                    }
                }),
                "http://127.0.0.1:18080/v1/jobs/one-shot"
            ]
            result = subprocess.run(curl_cmd, capture_output=True, text=True)
            if result.stdout.strip() == "201":
                success_count += 1

        elapsed = time.time() - start
        results["create_job"] = {
            "total_requests": 1000,
            "successful": success_count,
            "elapsed_seconds": elapsed,
            "requests_per_second": 1000 / elapsed
        }

    finally:
        proc.terminate()
        proc.wait()
        os.remove(binary_path)

    return results

def parse_wrk_output(output):
    """Parse wrk benchmark output."""
    result = {
        "requests_per_second": 0,
        "latency_avg_ms": 0,
        "latency_max_ms": 0,
        "total_requests": 0
    }

    # Parse requests/sec
    match = re.search(r'Requests/sec:\s+([\d.]+)', output)
    if match:
        result["requests_per_second"] = float(match.group(1))

    # Parse latency
    match = re.search(r'Latency\s+([\d.]+)(us|ms|s)\s+([\d.]+)(us|ms|s)\s+([\d.]+)(us|ms|s)', output)
    if match:
        avg_val = float(match.group(1))
        avg_unit = match.group(2)
        max_val = float(match.group(5))
        max_unit = match.group(6)

        # Convert to ms
        if avg_unit == "us":
            avg_val /= 1000
        elif avg_unit == "s":
            avg_val *= 1000

        if max_unit == "us":
            max_val /= 1000
        elif max_unit == "s":
            max_val *= 1000

        result["latency_avg_ms"] = avg_val
        result["latency_max_ms"] = max_val

    # Parse total requests
    match = re.search(r'(\d+)\s+requests in', output)
    if match:
        result["total_requests"] = int(match.group(1))

    return result

def generate_graphs(results):
    """Generate performance graphs."""
    if plt is None:
        print("Skipping graph generation (matplotlib not available)")
        return

    print("Generating performance graphs...")

    # Graph 1: Operations latency
    if "store" in results["benchmarks"] and results["benchmarks"]["store"]:
        store_benchmarks = results["benchmarks"]["store"]

        names = []
        latencies = []

        for name, data in store_benchmarks.items():
            short_name = name.replace("Benchmark", "")
            names.append(short_name)
            latencies.append(data["us_per_op"])

        if names:
            fig, ax = plt.subplots(figsize=(10, 6))
            bars = ax.barh(names, latencies, color=['#3498db', '#2ecc71', '#e74c3c', '#9b59b6', '#f39c12'][:len(names)])
            ax.set_xlabel('Latency (μs)')
            ax.set_title('Redis Operations Latency')
            ax.set_xlim(0, max(latencies) * 1.2)

            # Add value labels
            for bar, val in zip(bars, latencies):
                ax.text(val + max(latencies) * 0.02, bar.get_y() + bar.get_height()/2,
                       f'{val:.1f} μs', va='center', fontsize=10)

            plt.tight_layout()
            plt.savefig(os.path.join(DOCS_IMG_DIR, 'redis_latency.png'), dpi=150)
            plt.close()
            print(f"  Created: redis_latency.png")

    # Graph 2: Operations throughput
    if "store" in results["benchmarks"] and results["benchmarks"]["store"]:
        store_benchmarks = results["benchmarks"]["store"]

        names = []
        throughput = []

        for name, data in store_benchmarks.items():
            short_name = name.replace("Benchmark", "")
            names.append(short_name)
            throughput.append(data["ops_per_sec"])

        if names:
            fig, ax = plt.subplots(figsize=(10, 6))
            bars = ax.barh(names, throughput, color=['#27ae60', '#3498db', '#e67e22', '#8e44ad', '#c0392b'][:len(names)])
            ax.set_xlabel('Operations per second')
            ax.set_title('Redis Operations Throughput')
            ax.set_xlim(0, max(throughput) * 1.2)

            # Add value labels
            for bar, val in zip(bars, throughput):
                ax.text(val + max(throughput) * 0.02, bar.get_y() + bar.get_height()/2,
                       f'{val:,.0f} ops/s', va='center', fontsize=10)

            plt.tight_layout()
            plt.savefig(os.path.join(DOCS_IMG_DIR, 'redis_throughput.png'), dpi=150)
            plt.close()
            print(f"  Created: redis_throughput.png")

    # Graph 3: Memory and Binary size
    fig, axes = plt.subplots(1, 2, figsize=(12, 5))

    # Binary size
    binary_mb = results["binary_size"]["mb"]
    ax1 = axes[0]
    ax1.bar(['TickHook Binary'], [binary_mb], color='#3498db', width=0.5)
    ax1.set_ylabel('Size (MB)')
    ax1.set_title('Binary Size (stripped)')
    ax1.set_ylim(0, binary_mb * 1.5)
    ax1.text(0, binary_mb + 0.5, f'{binary_mb:.1f} MB', ha='center', fontsize=12, fontweight='bold')

    # Memory usage
    rss_mb = results["memory"]["rss_mb"]
    ax2 = axes[1]
    ax2.bar(['RSS Memory'], [rss_mb], color='#2ecc71', width=0.5)
    ax2.set_ylabel('Memory (MB)')
    ax2.set_title('Memory Footprint (at startup)')
    ax2.set_ylim(0, max(rss_mb * 1.5, 50))
    ax2.text(0, rss_mb + 1, f'{rss_mb:.1f} MB', ha='center', fontsize=12, fontweight='bold')

    plt.tight_layout()
    plt.savefig(os.path.join(DOCS_IMG_DIR, 'footprint.png'), dpi=150)
    plt.close()
    print(f"  Created: footprint.png")

    # Graph 4: API Performance
    if "api" in results and results["api"]:
        fig, ax = plt.subplots(figsize=(10, 6))

        metrics = []
        values = []

        if "health" in results["api"] and results["api"]["health"]["requests_per_second"] > 0:
            metrics.append("Health Check\n(req/s)")
            values.append(results["api"]["health"]["requests_per_second"])

        if "create_job" in results["api"]:
            metrics.append("Create Job\n(req/s)")
            values.append(results["api"]["create_job"]["requests_per_second"])

        if metrics:
            bars = ax.bar(metrics, values, color=['#3498db', '#2ecc71'])
            ax.set_ylabel('Requests per second')
            ax.set_title('API Performance')

            for bar, val in zip(bars, values):
                ax.text(bar.get_x() + bar.get_width()/2, val + max(values) * 0.02,
                       f'{val:,.0f}', ha='center', fontsize=12, fontweight='bold')

            plt.tight_layout()
            plt.savefig(os.path.join(DOCS_IMG_DIR, 'api_performance.png'), dpi=150)
            plt.close()
            print(f"  Created: api_performance.png")

def generate_summary(results):
    """Generate a markdown summary of results."""
    summary = []
    summary.append("## Performance Metrics Summary\n")
    summary.append(f"*Generated on {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}*\n\n")

    # Binary Size
    summary.append("### Binary Size\n")
    summary.append(f"- **Stripped binary**: {results['binary_size']['mb']:.2f} MB\n\n")

    # Memory Footprint
    summary.append("### Memory Footprint\n")
    summary.append(f"- **RSS at startup**: {results['memory']['rss_mb']:.1f} MB\n")
    summary.append(f"- **Virtual memory**: {results['memory']['vms_mb']:.1f} MB\n\n")

    # Redis Operations
    if results["benchmarks"]["store"]:
        summary.append("### Redis Operations Performance\n")
        summary.append("| Operation | Latency | Throughput |\n")
        summary.append("|-----------|---------|------------|\n")

        for name, data in results["benchmarks"]["store"].items():
            short_name = name.replace("Benchmark", "")
            summary.append(f"| {short_name} | {data['us_per_op']:.2f} μs | {data['ops_per_sec']:,.0f} ops/s |\n")
        summary.append("\n")

    # API Performance
    if results.get("api"):
        summary.append("### API Performance\n")
        if "health" in results["api"] and results["api"]["health"]["requests_per_second"] > 0:
            h = results["api"]["health"]
            summary.append(f"- **Health Check**: {h['requests_per_second']:,.0f} req/s (avg latency: {h['latency_avg_ms']:.2f}ms)\n")

        if "create_job" in results["api"]:
            c = results["api"]["create_job"]
            summary.append(f"- **Create Job**: {c['requests_per_second']:,.0f} req/s\n")
        summary.append("\n")

    return "".join(summary)

def main():
    ensure_dirs()

    results = {
        "timestamp": datetime.now().isoformat(),
        "benchmarks": {},
        "binary_size": {},
        "memory": {},
        "api": {}
    }

    # Run benchmarks
    results["benchmarks"] = run_go_benchmarks()
    results["binary_size"] = measure_binary_size()
    results["memory"] = measure_memory_usage()

    # API benchmarks (optional, requires server running)
    try:
        results["api"] = run_api_benchmark()
    except Exception as e:
        print(f"API benchmark failed: {e}")
        results["api"] = {}

    # Save results
    with open(BENCH_RESULTS_FILE, "w") as f:
        json.dump(results, f, indent=2)
    print(f"\nResults saved to: {BENCH_RESULTS_FILE}")

    # Generate graphs
    generate_graphs(results)

    # Generate summary
    summary = generate_summary(results)
    print("\n" + summary)

    # Save summary
    summary_file = os.path.join(PROJECT_ROOT, "BENCHMARK_RESULTS.md")
    with open(summary_file, "w") as f:
        f.write(summary)
    print(f"Summary saved to: {summary_file}")

if __name__ == "__main__":
    main()
