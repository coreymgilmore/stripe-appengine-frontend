# How to install and get this app running

### Golang
1. Install go (golang).
    * https://golang.org/doc/install
1. Make sure you have a correctly functioning GOPATH.
    * Run `go env` to see where your GOPATH is set to.
    * Make sure you can browse to this path.

### Google Account & Cloud
1. Make sure you have a Google account.  If not, create one.
2. Make sure you can access the Goolge Cloud Platform console.
    * https://console.cloud.google.com/
3. Create a new project in the Google Cloud.
    * Select Go as the language.
    * Select the proper region (closest to your location).

### Stripe Account
1. Make sure you have a Stripe account.  If not, create one.
    * https://stripe.com/
2. Make sure your account is active and can accept payments.
3. Get your API keys.
    * Log in to the Stripe Dashboard.
    * https://dashboard.stripe.com/account/apikeys.
    * Choose test or live keys as needed.

### Google Cloud SDK
1. Install the Google Cloud SDK.
    * https://cloud.google.com/appengine/docs/standard/go/download
    * Choose the defaults through the install.
    * Initialize as required.
2. Install the appengine components.
    * Open the Google Cloud SDK terminal.
    * Run `gcloud components install app-engine-go`.
3. Update Google Cloud SDK if needed.
    * Open the Google Cloud SDK terminal.
    * Run `gcloud components update`.

### Download & config this app.
1. Either download a zip file from Github or clone this repo.
    * Extract or clone to the directory $GOPATH/src/github.com/coreymgilmore/stripe-appengine-frontend/
    * You must use this directory so that all go import work properly.
2. Open a terminal in the directory you downloaded or cloned the files to.
3. Change directory to ./services/process-cards/
4. Run `go get -u -v ./...` to download all the dependencies of this app.
    * This will take some time.
    * This must complete successfully.
5. Configure the `app.yaml` file.
    * Copy the `app.yaml.template` file to `app.yaml`.
    * Edit the `app.yaml` file.
        * Set `SESSION_AUTH_KEY` to a 64 character random string.
        * Set `SESSION_ENCRYPT_KEY` to a 32 character random string.
        * Set `STRIPE_SECRET_KEY` to your Stripe secret key.  It starts with "sk_".
6. Configure the `stripe-public-key.js` file.
    * Copy the `stripe-public-key.js.template` file to `stripe-public-key.js`.
    * Edit the `stripe-public-key.js` file to use your Stripe publishable key.  It starts with "pk_".

### Deploy the app to App Engine.
1. Open a terminal in the location of the `app.yaml` file.
2. Run `gcloud projects list` and make sure the project you created is in the list.
3. Run `gcloud app deploy --promote --project [YOUR_PROJECT_ID]`.
    * Exchange `--promote` with `--no-promote` to stop this version of the app from automatically taking over traffic.  Do this when you want to test the new version first.
    * Add `--version [YOUR_VERSION_ID]` if you want to give this version of the app your own version number.  Otherwise you will be assigned a version number based on the current datetime.
    * You will see a confirmation prompt.  Click 'y'.
4. You will also need to upload the indexes: `gcloud app deploy index.yaml`.
5. When the deployment is complete you will be able to use the app on the `https://[YOUR-PROJECT-ID].appspot.com`.