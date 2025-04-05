# Basic usage (GET)
burl https://example.com

# POST with data
burl -X POST -d "user=admin&pass=123" https://httpbin.org/post

# Verbose and save to file
burl -v -i -o out.html -H headers.txt https://target.com

# Follow redirects
burl -L https://site-with-redirects.com
