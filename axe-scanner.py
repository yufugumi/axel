import asyncio
import argparse
from axe_scanner_lib import run_accessibility_scan

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Run accessibility scans')
    parser.add_argument('--site', type=str, required=True, 
                        choices=['wellington', 'letstalk', 'archives', 'transportprojects', 'careers'],
                        help='Which site to scan')
    
    args = parser.parse_args()
    
    # Site-specific configurations
    site_configs = {
        'wellington': {
            'url_file': 'wellington.txt',
            'test_name': 'wellington-govt-nz',
            'log_file': 'wellington.log',
            'exclude_rules': ['duplicate-id-active']
        },
        'letstalk': {
            'url_file': 'letstalk.txt',
            'test_name': 'lets-talk',
            'log_file': 'letstalk.log'
        },
        'archives': {
            'url_file': 'archives.txt',
            'test_name': 'archives-online',
            'log_file': 'archives.log'
        },
        'transportprojects': {
            'url_file': 'transportprojects.txt',
            'test_name': 'transportprojects',
            'log_file': 'transportprojects.log'
        },
        'careers': {
            'url_file': 'careers.txt',
            'test_name': 'careers',
            'log_file': 'careers.log'
        }
    }
    
    config = site_configs[args.site]
    asyncio.run(run_accessibility_scan(**config))
