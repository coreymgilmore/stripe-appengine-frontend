# An App for Collecting and Charging Credit Cards

#### Changelog:
Version 3 (Late-January 2018) has breaking changes.

#### Intro:
This app is designed for business that collect credit cards and charge them upon receiving customer orders.  The best use case if for a company that sells most of its goods via phone order, purchase orders, etc. versus e-commerce.  For example: an AP assistant provides a credit card but a purchasing supervisor sends over purchase orders or calls in an order.  This stops the company who is selling goods from constantly needing to reaquire a credit card for payment (the typical "we will keep this card on file").

The hosting of this app is done through Google App Engine.  The install/deployment of this app is done by an IT professional but usage of this app only requires minimal skill.  App Engine required very little to no management.

Access to this app is via a web browser.  You can use this app from any web connected device in any location.

All processing and storing of credit card data is done via Stripe (https://stripe.com).  Since this uses Stripe you get the benefits of vastly easier to understand pricing, a modern administrative interface, and the ability to store cards without needing PCI compliance or worrying about someone hacking your database.  You will no longer need pay for PCI compliance or keep copies of credit cards in your database, on paper, or anywhere else.


***

Please see the [wiki pages](https://github.com/coreymgilmore/stripe-appengine-frontend/wiki) for in-depth install, usage, and other details.

***

#### What can you do with this app?:
1. Add credit cards to charge now or in the future.
2. Remove cards that already exist.
3. Charge credit cards and refund charges.
4. View transaction reports (list of charges and refunds).
5. Add or remove users as needed.
6. Control users' permissions to add, remove, charge cards, and view reports.
8. Set your own Statement Descriptor so your customers recognize your charge.
9. Print receipts.
10. Make API-style requests to autofill the card, amount, invoice, and purchase order.

#### Who should use this app?:
- Any company who processes credit cards via a virtual terminal.
- Any company who processes non-ecommerce style orders.
- Any company who saves customers' cards and then processes the card when the customer places an order.
- Best used by companies who still receive orders via phone, email, or fax or where a card may be a corporate card and the purchasing person does not know of it.

#### Benefits over other virtual terminals:
- Does not require PCI compliance.
- No storing of credit card information on your servers.
- Simple, clean, easy to use, and modern interface.
- You control the user's and access rights.
- Simple, most likely cheaper, pricing via Stripe.
- Very quick deposits to your bank acount.
- Secure.

#### How it works:
1. You create a new customer by providing the customer's name and card information.
2. The card data is saved to Stripe and an ID is saved to the app.
3. When you want to charge a card the ID is set to Stripe.
4. Stripe looks up the credit card's information and processes the charge.
5. If the charge is successful, a receipt is shown.  If the card was declined, an error is shown.
6. Print a receipt or view a daily transaction log.


#### Limitations:
- Currency is currently hardcoded as USD.
    - This can be changed in card/card.go as the currency constant.
    - The currency symbol would also need to be changed in the app.  This only affects the user interface though.
- You can only store one credit card per customer.
- This app *only* works on App Engine, not in a normal Golang environment.

#### Install & Setup:
- Please see `INSTALL.md for more thorough instructions.

1. Install Go.
2. Create a Google Cloud project.
3. Install the Google Cloud SDK.
4. Create a Stripe account.
5. Download this app.
6. Configure.
7. Deploy.
8. Run it!


#### Pricing:

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

***

#### Contributing, Issues, New Feature
Create an issue or a pull request!