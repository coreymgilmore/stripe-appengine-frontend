Data stored in memcache:
************************

Customer Card Data:
	- Key: customer id (unique key given during customer creation)
		- If this was given when creating the customer.
		- Used for api-style autoloading of card data into charge card form.
		- If you do not use the api-style link to autofill the charge card form, then this data is not stored in memcache.
	- Value:
		- Customer name
		- Cardholder
		- Card's last 4 digits
		- Card's expirtation


Customer Card Data:
	- Key: card's datastore IntID
		- This is used when looking up card data for charging cards or removing cards.
		- Saved and used every time a card is used.
	- Value:
		- Customer id
		- Customer name
		- Cardholder
		- Card's last 4 digits
		- Card's expiration
		- Stripe customer token


List of Cards:
	- Key: "list-of-cards"
		- hardcoded
	-Value:
		- Map of structs.
		- Each struct has the card's customer name and datastore IntID.
		- Used for building list of customer cards to charge or remove.


List of Users
	- Key: "list-of-users"
		- hardcoded
	- Value:
		- Map of structs.
		- Each struct has the user's username and datastore IntID.
		- Used for building list of users when editing users' settings.


User Data
	- Key: user's datastore IntID
		- Used when looking up a customer to change password or settings.
		- Used when looking up permissions from the session.
	- Value
		- All user data.


User Data
	- Key: username (email address)
		- Used when a user logs in.
	- Value:
		- All user data.