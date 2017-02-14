#A Credit Card Processing Virtual Terminal

####Changelog:
Please see the changelog.txt file if you used version 1 (prior to February 2017).  Version 2 (Feb. 14, 2017 and forward) has breaking changes.

####Intro:

This app is basically a "virtual terminal" for collection and charging credit cards.  This is meant to be used for businesses that collect credit cards from customers and then charge this card repeadly for orders.  It removes the need to ask for card information at every purchase, stops you from storing credit cards in your database or on paper, and does not require PCI compliance since you are not storing any data about the cards.

All processing and storing of credit card data is done via Stripe (https://stripe.com).  Since this uses Stripe you get the benefits of vastly easier to understand pricing, a modern administrative interface, and the ability to store cards without needing PCI compliance or worrying about someone hacking your database.

When you install this app, you deploy it to Google App Engine. The app lives "in the cloud" so your employees can access it from anywhere.  This also reduces complexity and the needs for IT personel.

***

Please see the [wiki pages](https://github.com/coreymgilmore/stripe-appengine-frontend/wiki) for in-depth install, usage, and other details.

***

####Quick Screenshots:
![charge-card](https://raw.githubusercontent.com/coreymgilmore/stripe-appengine-frontend/master/doc_imgs/charge-card.png)
![add-card](https://raw.githubusercontent.com/coreymgilmore/stripe-appengine-frontend/master/doc_imgs/add-card.png)

####What can you do with this app?:
1. Add credit cards to charge now or in the future.
2. Remove cards that already exist.
3. Charge credit cards (and refund charges).
4. View transaction reports.
5. Add or remove user's as needed.
6. Control users' permissions to add, remove, or charge cards, and view reports.
7. Block a user.
8. Set your own Statement Descriptor so your customers recognize your charge.
9. Print receipts.
10. Make API-style requests to autofill the card, amount, invoice, and purchase order.

####Who should use this app?:
- Any company who processes credit cards via a virtual terminal.

####Benefits over other virtual terminals:
- Does not require PCI compliance.
- No storing of credit card information on your servers.
- Simple, clean, easy to use, and modern interface.
- You control the user's and access rights.
- Simple pricing via Stripe.
- Very quick deposits to your bank acount. As short as 1-day.

####How it works:
1. You create a new customer by providing the customer's name and card information.
2. The card data is saved to Stripe and an ID is saved to the app.
3. When you want to charge a card, the ID is used to send the charge to Stripe.
4. Stripe looks up the credit card's information and processes the charge.
5. If the charge is successful, a receipt is shown.  If the card was declined, an error is shown.


####Limitations:
- Currency is currently hardcoded as USD.
  - This can be changed in card/card.go as the currency constant.
- You *must* access this app over HTTPS.
  - Stripe requires this for security and it makes absolute sense.
  - If you use this app with the *.appspot.com URL, your app can be accessed by HTTPS without any changes.
- Only the basic company information and address are editable on the receipt.
- You can only store one credit card per customer.
- This app *only* works on App Engine, not in a normal Golang environment.

####Install & Setup:
- Please see the [wiki page](https://github.com/coreymgilmore/stripe-appengine-frontend/wiki/Install-&-Setup) for more thorough instructions.

1. Create a Stripe account.
2. Activate your Stripe account.
3. Create a Google Account.
4. Create an App Engine project.
5. Download this app's source code.
6. Configure the app.
7. Deploy to App Engine.
8. Done!

####Pricing:

This app is 100% free to install and use.  However, the processing of credit cards and hosting are not free. There are two pricing considerations for this app. Stripe, and Google App Engine.  Please see the [wiki page](https://github.com/coreymgilmore/stripe-appengine-frontend/wiki/Costs-of-Using-this-App) that details the costs to use this app.

***

This app uses the following:
- [Boostrap](http://getbootstrap.com/) - Basic layout and html elements.
- [jQuery](https://jquery.com/) - Javascript library.
- [Stripe](https://stripe.com/) - Payment processing.
- [Google App Engine](https://cloud.google.com/appengine/docs) - Hosting platform.
- [Golang](https://golang.org/) - Backend programming language, web server.
- [Alice](https://github.com/justinas/alice) - Go middleware handler.
- [Gorilla Mux](https://github.com/gorilla/mux) - Go http router.
- [Gorilla Sessions](https://github.com/gorilla/sessions) - Secure sessions.
- [Timestamps](https://github.com/coreymgilmore/timestamps) - Standardized timestamp creation.
- [Pwds](https://github.com/coreymgilmore/pwds) - Simple wrapper functions to create/verify passwords.
