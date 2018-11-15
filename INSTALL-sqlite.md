# Deployment via Anything but App Engine
Run anywhere that supports golang >=1.11, serving a website, and sqlite3.
*Tested on Windows 10 and Ubuntu 16.04.*

### Golang
1. Install golang >= 1.11.
    * https://golang.org/doc/install
2.. Make sure you have a correctly functioning GOPATH.
    * Run `go env` to see where your GOPATH is set to.
    * Make sure you can browse to this path.

### Stripe Account
1. Make sure you have a Stripe account.  If not, create one.
    * https://stripe.com/
2. Make sure your account is active and can accept payments.
3. Get your API keys.
    * Log in to the Stripe Dashboard.
    * https://dashboard.stripe.com/account/apikeys.

### Install SQLite 3
*You will need to install SQLite on the system you intend to run this app.  You may also want to install it on your desktop for testing.*
*If running on Windows, make sure you have GCC installed.  See this [link](https://medium.com/@yaravind/go-sqlite-on-windows-f91ef2dacfe) for help.*
1. Search the web for instructions on how to install for your OS.
2. Make sure SQLite works by running `sqlite3` in your terminal.

### Download & Config this App.
1. Get the source code to this app.
    * Download zip file from Github...
        * Extract the contents to the $GOPATH/src/github.com/coreymgilmore/stripe-appengine-frontend/ directory on your computer.
        * The path to main.go should be $GOPATH/src/github.com/coreymgilmore/stripe-appengine-frontend/services/process-cards/app.yaml  
    * Go get...
        * Run `go get github.com/coreymgilmore/stripe-appengine-frontend/...` in your terminal.
        * The files should have be downloaded to $GOPATH.
    * Using the `$GOPATH/src/github.com/coreymgilmore/stripe-appengine-frontend/` directory is very important!        
3. In a terminal, navigate to the `./services/process-cards/` directory.
4. Run `go get -u -v ./...` to download all the dependencies of this app.
    * This will take some time.
    * This must complete successfully.
5. Configure the app.yaml file.
    * Browse to `./services/process-cards/` and fine the app.yaml.template file.
    * Copy the app.yaml.template file to app.yaml.
    * You can move this file to any location that suits you.
    * Edit the app.yaml file.
        * Set `SESSION_AUTH_KEY` to a 64 character random string.
        * Set `SESSION_ENCRYPT_KEY` to a 32 character random string.
        * Set `STRIPE_SECRET_KEY` to your Stripe secret key.  It starts with "sk_".
        * Set `STRIPE_PUBLISHABLE_KEY` to your Stripe publishable key.  It starts with "sk_".
        * Set `PATH_TO_STATIC_FILES` to the full path to the `./stripe-appengine-frontend/services/process-cards/website/static/` directory.
        * Set `PATH_TO_TEMPLATES` to the full path to the `./stripe-appengine-frontend/services/process-cards/templates/` directory.
        * Set `PATH_TO_SQLITE_FILE` to the full path to where you want to store the `sqlite.db` file if you don't want to use the default.
            * *Defaults to `$GOPATH/src/github.com/coreymgilmore/stripe-appengine-frontend/services/process-cards/sqlite.db` if you don't set it.
        * On Windows, use "/" instead of "\" or escape the "\" characters in the paths.
        * Use live or test Stripe keys.  Just understand what each is for.  
        * All other items can be left as-is or modified based on the comments in the app.yaml file.     
        * You will have to redeploy this app anytime you change a value in this file.

### Web Proxy, DNS, SSL/TLS Certificate & Firewall
* By default, this app serves on port 8005.  You can change this with the `PORT` environmental variable.  Configure a web proxy to serve this website on port 443 with HTTPS so that users do not have to type in a port number.
* Configure DNS so that users do not have to type in an IP address to access this website.
* Make sure you have a valid SSL/TLS certificate for the domain you will serve this app from otherwise cards will not be able to be stored.
* Configure the firewall on the system this app is installed in to allow traffic to the port this app is serving on.

### Install & Run.
1. Go to the `/stripe-appengine-frontend/services/process-cards/` directory.
2. Run `go install` and make sure no errors occured.
    * Make sure you have `$GOPATH/bin` in your $PATH (unless you want to run the executable using the full path to it).
3. Run the app by running `process-cards --type=sqlite --path-to-app-yaml="/full/path/to/your/modified/app.yaml"`
    * Use `process-cards.exe` on Windows.
    * The app should start and output "Starting stripe-appengine-frontend..." and a few other diagnostic messages.
4. Browse to the domain name you set or the IP of the system this app is running on (plus the PORT as needed).

### Initial Log In & In App Settings
1. Upon first browsing to the app, you will need to create the super-admin password.
    * This user should rarely, if ever, be used.  It will only be used to create your initial user.
2. Warning about company settings for receipt and statement descriptor will be shown.  Set them.
    * Click "Go" under Change Company Info.
    * Provide your company's info and click "Save".
    * Refresh your browser.
3. Create your users.
4. Log out of the super-admin and log in as a normal user and start using the app.

### Run Automatically
* Set up your system to run the `process-cards --type=...` command automatically and save any output to a log file.
* `systemctl`, `init.d`, etc. on non-Windows systems.

### Diagnostics About the App
1. Output is logged to the terminal or a log file if you configured it.
3. Check the logs.

### Diagnostics About Charges
1. Log in to the Stripe Dashboard.