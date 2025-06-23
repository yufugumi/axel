import asyncio
import os
from playwright.async_api import async_playwright
from axe_core_python.async_playwright import Axe
from tqdm.asyncio import tqdm
from datetime import datetime
from jinja2 import Template
import logging
from typing import List, Dict, Tuple

# Ensure required directories exist
for directory in ['reports', 'logs', 'urls']:
    os.makedirs(directory, exist_ok=True)

async def process_url(url: str, browser, semaphore, axe: Axe, progress_bar) -> Tuple[str, List[Dict]]:
    """Process a single URL with error handling and retries."""
    max_retries = 2
    retry_count = 0
    
    while retry_count <= max_retries:
        try:
            async with semaphore:  # Limit concurrent connections
                context = await browser.new_context(
                    viewport={'width': 1280, 'height': 720},
                    user_agent='WAXE accessibility testing bot'
                )
                
                try:
                    # Block Google Analytics and GTM requests
                    await context.route('**/*google-analytics*', lambda route: route.abort())
                    await context.route('**/*googletagmanager*', lambda route: route.abort())
                    await context.route('**/*gtm.js*', lambda route: route.abort())
                    await context.route('**/*analytics.js*', lambda route: route.abort())
                    await context.route('**/*ga.js*', lambda route: route.abort())
                    
                    page = await context.new_page()
                    await page.goto(url, timeout=30000, wait_until='networkidle')
                    
                    # Add code to block GTM iframes
                    await page.add_script_tag(content='''
                    (function() {
                        const removeGTMIframes = () => {
                            const iframes = document.querySelectorAll('iframe');
                            iframes.forEach(iframe => {
                                if (iframe.src && (
                                    iframe.src.includes('googletagmanager') || 
                                    iframe.src.includes('gtm') ||
                                    iframe.src.includes('google-analytics')
                                )) {
                                    iframe.remove();
                                }
                            });
                        };
                        
                        removeGTMIframes();
                        
                        const observer = new MutationObserver((mutations) => {
                            removeGTMIframes();
                        });
                        
                        observer.observe(document.documentElement, {
                            childList: true,
                            subtree: true
                        });
                    })();
                    ''')
                    
                    # Wait for page to be fully loaded
                    await page.wait_for_load_state('networkidle')

                    await asyncio.sleep(2)
   
                    # Run accessibility tests
                    result = await axe.run(page)
                    
                    if result.get('violations'):
                        progress_bar.write(f"{len(result['violations'])} violations found on {url}")
                    else:
                        progress_bar.write(f"No violations found on {url}")
                    
                    return url, result.get('violations', [])
                
                finally:
                    await context.close()  # Ensure context is closed to free resources
        
        except Exception as e:
            retry_count += 1
            if retry_count <= max_retries:
                progress_bar.write(f"Error on {url}, retrying ({retry_count}/{max_retries}): {e}")
                await asyncio.sleep(2)  # Wait before retry
            else:
                progress_bar.write(f"Failed to process {url} after {max_retries} retries: {e}")
                return url, []
        finally:
            progress_bar.update(1) 

async def process_urls(urls: List[str]) -> List[Tuple[str, List[Dict]]]:
    """Process all URLs with a global progress bar."""
    axe = Axe(options={
        "tags": ["wcag22aa"]  
    })
    results = []
    
    max_concurrent = 10  
    semaphore = asyncio.Semaphore(max_concurrent)
    
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(headless=True)
        
        try:
            # Create a single progress bar for all URLs
            with tqdm(total=len(urls), desc="Processing URLs") as progress_bar:
                # Process URLs in smaller chunks to manage memory while maintaining the global progress bar
                chunk_size = 20  # Adjust based on memory constraints
                for i in range(0, len(urls), chunk_size):
                    chunk = urls[i:i + chunk_size]
                    
                    # Create tasks for this chunk
                    tasks = [
                        process_url(url, browser, semaphore, axe, progress_bar) 
                        for url in chunk
                    ]
                    
                    # Wait for all tasks in this chunk to complete
                    chunk_results = await asyncio.gather(*tasks)
                    results.extend(chunk_results)
                    
                    await asyncio.sleep(2)
                    
        finally:
            await browser.close()
            
    return results

async def run_accessibility_scan(url_file: str, test_name: str, log_file: str = "accessibility_scan.log"):
    """Run an accessibility scan with the specified parameters."""
    # Setup logging
    full_log_path = os.path.join('logs', log_file)
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(levelname)s - %(message)s',
        handlers=[
            logging.FileHandler(full_log_path),
            logging.StreamHandler()
        ]
    )
    logger = logging.getLogger(__name__)
    
    now = datetime.now()
    date_string = now.strftime("%d-%m-%Y")
    
    # Read URLs from urls directory
    full_url_path = os.path.join('urls', url_file)
    with open(full_url_path, 'r') as file:
        urls = [url.strip() for url in file if url.strip()]
        
    logger.info(f"Starting accessibility scan of {len(urls)} URLs")
    
    # Process all URLs with a global progress bar
    results = await process_urls(urls)
    
    # Read template
    with open('template.html') as f:
        template = Template(f.read())
    
    # Render HTML report
    html_output = template.render(
        date=date_string,
        results=results,
        test_name=test_name
    )
    
    # Save report to reports directory
    report_filename = f'accessibility-report-{test_name}-{date_string}.html'
    full_report_path = os.path.join('reports', report_filename)
    with open(full_report_path, 'w', encoding='utf-8') as f:
        f.write(html_output)
        
    logger.info(f"Accessibility report generated: {full_report_path}")