# Webapp for Storing and Charging Credit Cards

#### Intro:
This application (webapp) is designed for business that collect credit cards, store them, and charge them upon receiving orders.  The designed-for use case is a company that collects orders via phone or email, not ecommerce.  Think more manual order entry versus someone picking items on a website.  This provides the "we will keep this card on file" functionality.

All processing and storing of credit cards is done via [Stripe](https://stripe.com/).  Using Stripe provides you the benefits of simple and reasonable pricing, a modern administrative interface, and PCI compliance without any work on your side.  You no longer need to worry about storing card information securely, someone hacking your internal systems, or paying for PCI compliance.

#### Changelog:
Version 4 (early-November 2018) is a major change.  See [changelog.txt](changelog.txt). See the install docs for instructions.

Version 5 (mid-November 2018) is a minor change for user's deploying via App Engine.  However, this is a major release as you can now run this app without App Engine.

#### Install:
This application is designed to run via:
- [Google App Engine](https://cloud.google.com/appengine/).  
    - [Install docs.](INSTALL-appengine.md)
- Any system (computer/server/VM) capable of serving a website and running [golang](https://golang.org/) >=1.11 and [SQLite](https://sqlite.org/index.html).
    - [Install docs.](INSTALL-sqlite.md)

#### What can you do with this app?:
1. Add credit cards.
2. Charge credit cards and refund charges.
3. View transaction reports (list of charges and refunds).
4. Add or remove users of the application as needed.
5. Control users' permissions to add, remove, charge cards, and view reports.
6. Set your own Statement Descriptor so your customers recognize your charge on their statements.
7. Print receipts.
8. Integrate into your other systems/applications by making API requests to autofill the charge form or automatically charge a card.

#### Who should use this app?:
- Companies who processes non-ecommerce style orders.
- Companies who gather payment information from an AP department but orders from a purchasing department.
- Companies who saves customers' cards and then processes the card when the customer places an order.
- Companies who charge cards but want to reduce their IT complexity and/or PCI compliance costs.
- Companies who are paying more than Stripe's [pricing.](https://stripe.com/us/pricing)

#### How it works:
1. You store a new card providing the customer's name and card information.
2. The card data is saved to Stripe and an ID is saved to this application.
3. When you want to charge a card the ID is set to Stripe.
4. Stripe looks up the credit card's information and processes the charge.
5. If the charge is successful, a receipt is shown.  If the card was declined, an error is shown.
6. Print a receipt or view a daily transaction log.

#### Limitations:
- Currency is currently hardcoded as USD (as is the $ symbol).
- You can only store one credit card per customer.

***

#### Technical Stuff & FAQs:
1. The app.yaml file is used for all configuration and deployment options.  This makes deployment simple: users don't need to understand source code, they just have to change some text in one file.
2. This app uses the "generation 2" runtime on App Engine.  As of the beta (Nov. 2018), this allows for near-identical source code to a non-appengine app and is therefore the reason non-appengine deployments are now developed (sqlite).
3. This app uses Google Cloud Datastore as the database when deployed on App Engine.  This also uses Cloud Datastore for development as the locally running Datastore Emulator is a mess and pain to use (unless we need to generate indexes).
3. If running off of App Engine, SQLite is used as the database.  Why?  Because it is so easy to use and little to no install and/or configuration is required.
4. Caching using App Engine memcache is not supported as "gen 2" runtime doesn't support it.  Using RedisLabs as explained [here](https://cloud.google.com/appengine/docs/standard/go111/go-differences) has not been implemented due to small benefits of much more complex code.
5. Upgrading to a new version of this app is simple.  Follow the install docs to download the new source code and deploy.  If using App Engine you can provide a different version to separate the new and old apps.
6. Why aren't you using go modules or something else.  Because I like the GOPATH.

#### Integration with Other Applications:
* Autofill the charge card form:
    * Build this url `...my-app.appspot.com/main/?customer_id=<>&amount=<>&invoice=<>&po=<>` where...
    * `my-app.appspot.com` is the url you use to access your version of this app.
    * `customer_id` is the unique ID you use to identify customers in this app.  It would be smart to match this to an ID in your CRM or other software.
    * `amount` is the value in cents to charge.
    * `invoice` and `po` are optional and provide more information on the receipt when a charge is processed.
* Automatically charge a card:
    * Make sure you have an API key.  Check the App Settings under Settings within the application.
    * Build a POST request to `...my-app.appspot.com/card/auto-charge/` where the data sent is...
    * `customer_id` is the unique ID you use to identify customers in this app.
    * `amount` is the value in cents to charge.
    * `invoice` and `po` are optional and provide more information on the receipt when a charge is processed.
    * `api_key` is the API key as it shows in the app settings.
    * `auto_charge` is a simple check value that is set to true.  This is set to false when testing integration of this app.
    * `auto_charge_referrer` is the name of the system/program/application making the request to this app.  This is used for diagnostics/logging/reports.
    * `auto_charge_reason` is the name of the function within the system/program/application that is making the request to this app.  This is used for diagnostics/logging/reports.
    * `level3_provided` (optional) is set to true if level 3 charge data is provided in level3_params.
    * `level3_params` (optional) is set to the level 3 data for a charge.  This is an object with data about the charge plus an array with data for each line item on an order.  See [here](https://stripe.com/docs/level3) for details although this link will only work if you have been invited to try the private beta of level 3 charges (contact Stripe support).

***

#### Contributing, Issues, New Feature:
Create an issue or a pull request.
