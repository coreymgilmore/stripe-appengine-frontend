# Deployment via Google App Engine

### Golang
1. Install golang >= 1.11.
    * https://golang.org/doc/install
2.. Make sure you have a correctly functioning GOPATH.
    * Run `go env` to see where your GOPATH is set to.
    * Make sure you can browse to this path.

### Google Account & Cloud
1. Make sure you have a Google account.  If not, create one.
2. Make sure you can access the Googe Cloud Platform console.
    * https://console.cloud.google.lcom/
3. Create a new project in the Google Cloud.
    * Give the project a descriptive name: MyCompany-CreditCard.
    * Open the App Engine settings in the Cloud Platform admin console.
    * Create an application.
    * Choose the region closest to your users.
    * Choose Go as the language.
    * Choose Standard as the environment.

### Google Cloud SDK
1. Install the Google Cloud SDK.
    * https://cloud.google.com/appengine/docs/standard/go/download
    * Choose the defaults through the install.
    * Initialize as required.
2. Make sure you can access the `gcloud` command via your terminal.
    * Configure your PATH as needed.
3. Install the gcloud App Engine components.
    * Run `gcloud components install app-engine-go` in your terminal.
4. Update Google Cloud SDK if needed.
    * Run `gcloud components update` in your terminal.

### Stripe Account
1. Make sure you have a Stripe account.  If not, create one.
    * https://stripe.com/
2. Make sure your account is active and can accept payments.
3. Get your API keys.
    * Log in to the Stripe Dashboard.
    * https://dashboard.stripe.com/account/apikeys.

### Download & Config this App.
1. Get the source code to this app.
    * Download zip file from Github...
        * Extract the contents to the $GOPATH/src/github.com/coreymgilmore/stripe-appengine-frontend/ directory on your computer.
        * The path to main.go should be $GOPATH/src/github.com/coreymgilmore/stripe-appengine-frontend/services/process-cards/app.yaml  
    * Go get...
        * Run `go get github.com/coreymgilmore/stripe-appengine-frontend/...` in your terminal.
        * The files should have be downloaded to $GOPATH.
    * Using the $GOPATH/src/github.com/coreymgilmore/stripe-appengine-frontend/ directory is very important!        
3. In a terminal, navigate to the ./services/process-cards/ directory.
4. Run `go get -u -v ./...` to download all the dependencies of this app.
    * This will take some time.
    * This must complete successfully.
5. Configure the app.yaml file.
    * Browse to ./services/process-cards/ and fine the app.yaml.template file.
    * Copy the app.yaml.template file to app.yaml.
    * Edit the app.yaml file.
        * Set `PROJECT_ID` to the name of the project you created on Google Cloud Platform.        
        * Set `SESSION_AUTH_KEY` to a 64 character random string.
        * Set `SESSION_ENCRYPT_KEY` to a 32 character random string.
        * Set `STRIPE_SECRET_KEY` to your Stripe secret key.  It starts with "sk_".
        * Set `STRIPE_PUBLISHABLE_KEY` to your Stripe publishable key.  It starts with "sk_".
        * Use live or test Stripe keys.  Just understand what each is for.  
        * All other items can be left as-is or modified based on the comments in the app.yaml file.     
        * You will have to redeploy this app anytime you change a value in this file.

### Deploy to App Engine.
1. Open a terminal in the directory of the app.yaml file.
2. Run `gcloud projects list`
    * Make sure the project you created is in the list.
    * Make sure the project name matches the `PROJECT_ID` in app.yaml.
3. Run `gcloud app deploy --promote --project [YOUR_PROJECT_ID]`.
    * Add `--version [YOUR_VERSION_ID]` if you want to give this version of the app your own version number.  Otherwise you will be assigned a version number based on the current datetime.
    * You will see a confirmation prompt.  Click 'y'.
4. Run `gcloud app deploy index.yaml` to upload the indexes needed for the database to work properly.
5. When the deployment is complete you will be able to use the app on the `https://[YOUR-PROJECT-ID].appspot.com`.

### Initial Log In & In App Settings
1. Upon first browsing to the app, you will need to create the super-admin password.
    * This user should rarely, if ever, be used.  It will only be used to create your initial user.
2. Warning about company settings for receipt and statement descriptor will be shown.  Set them.
    * Click "Go" under Change Company Info.
    * Provide your company's info and click "Save".
    * Refresh your browser.
3. Create your users.
4. Log out of the super-admin and log in as a normal user and start using the app.

###Diagnostics About the App
1. Log in to the Google Cloud Platform admin console.
2. Choose the correct project. 
3. Check the logs.

###Diagnostics About Charges
1. Log in to the Stripe Dashboard.