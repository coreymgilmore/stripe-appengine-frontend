//*******************************************************************************
//CONSTANTS
const MIN_PASSWORD_LENGTH = 8;
const BAD_PASSWORDS = [
	"password",
	"password1",
	"12345678",
	"123456789",
	"123123123",
	"00000000",
	"1234567890",
	"asdfasdf",
	"asdfghjkl",
	"testtest",
	"admin@example.com"
];

//*******************************************************************************
//STRIPE PUBLIC KEY
Stripe.setPublishableKey('pk_test_pKzD1QYPWJNrwuGTZ2k0HEkn');

//*******************************************************************************
//COMMON FUNCS

//ESCAPE STRINGS
//remove HTML characters/tags from strings used for saving data to db
//http://stackoverflow.com/questions/6234773/can-i-escape-html-special-chars-in-javascript
function escapeHTML(string) {
	return string
		.replace(/&/g, "&amp;")
		.replace(/</g, "&lt;")
		.replace(/>/g, "&gt;")
		.replace(/"/g, "&quot;")
		.replace(/'/g, "&#039;");
}

//REGEX MATCH EMAIL
function validateEmail(email) {
	var regex = /^(([^<>()[\]\\.,;:\s@\"]+(\.[^<>()[\]\\.,;:\s@\"]+)*)|(\".+\"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\])|(([a-zA-Z\-0-9]+\.)+[a-zA-Z]{2,}))$/;
	return regex.test(email);
}

//CHECK IF TWO PASSWORDS MATCH
function doWordsMatch (word1, word2) {
	if (word1 === word2) {
		return true;
	}

	return false;
}

//CHECK IF A PASSWORD IS LONG ENOUGH
function isLongPassword (password) {
	if (password.length < MIN_PASSWORD_LENGTH) {
		return false;
	}

	return true;
}

//CHECK IF A GIVEN PASSWORD IS IN THE LIST OF TOO EASY TO GUESS PASSWORDS
function isSimplePassword (password) {
	if (BAD_PASSWORDS.indexOf(password) !== -1) {
		return true;
	}

	return false;
}

//SHOW AN ERROR MESSAGE IN A FORM
//title is usually "error" or "warning"
//message is something descriptive about the error so the user can fix the problem
//parentElem is the parent html element where the <div #error-msg> is located
function showFormErrorMsg (title, message, parentElem) {
	var alert = "" + 
		"<div class='alert alert-danger'>" + 
			"<b>" + title + "</b> " + message + 
		"</div>";
	parentElem.find('#error-msg').html(alert);
	return;
}

//*******************************************************************************
//FUNCS

//SHOW SPECIFIC ACTION PANEL WHEN NAV BUTTONS ARE CLICKED
$('body').on('click', '.action-btn', function() {
	//get data attribute which tells us which panel to show
	var btn = 			$(this);
	var dataAction = 	btn.data("action");

	//make sure buttons arent disabled
	if (btn.attr("disabled") !== undefined) {
		console.log("btn disabled");
		return;
	}

	//make sure the button clicked isn't already showing the correct panel
	var showingPanel = 		$('.action-panels.show');
	var showingPanelId = 	showingPanel.attr('id');
	if (showingPanelId === dataAction) {
		return;
	}

	//generate selector of new panel to show
	var newPanel = 			"#" + dataAction;
	var newPanelElem = 		$(newPanel);

	//get width of action-panel container to define slide in/out distance
	//so the panel slides completely out of view before new panel slides in
	//width changes bases on browser window size
	var containerWidth = 	$('#action-panels-container').outerWidth()

	//slide the current panel out to the left
	//slide the new panel in to the right
	//disabled all the buttons when sliding is occuring
	var allBtns = $('.action-btn');
	allBtns.attr("disabled", true).children('input').attr("disabled", true);
	showingPanel.toggle('slide', {distance: containerWidth}, 600, function() {
		showingPanel.removeClass('show');

		newPanelElem.toggle('slide', 600, function() {
			newPanelElem.addClass('show');
			allBtns.attr("disabled", false);
		});
	});
});

//VALIDATE THE INITIAL ADMIN CREATE FORM
//make sure the passwords are strong and match
$('#create-init-admin').submit(function (e) {
	//get passwords from inputs
	var pass1 = $('#password1').val();
	var pass2 = $('#password2').val();

	//check if passwords match
	if (doWordsMatch(pass1, pass2) === false) {
		e.preventDefault();
		showFormErrorMsg("Error", "The passwords do not match.", $('#create-init-admin'));
		return false;
	}

	//make sure the password is long enough
	if (isLongPassword(pass1) === false) {
		e.preventDefault();
		showFormErrorMsg("Error", "Your password is too short. It must be at least " + MIN_PASSWORD_LENGTH + " characters.", $('#create-init-admin'));
		return false;
	}

	//make sure the passwords are not very easy to guess
	if (isSimplePassword(pass1) === true) {
		e.preventDefault();
		showFormErrorMsg("Error", "The password you provided is too simple. Please choose a better password.", $('#create-init-admin'));
		return false;
	}

	//passwords validated
	//form will submit and save admin user
	//user will see a success/error page
	//then user can log in on main login page
});

//GENERATE LIST OF YEARS FOR CARD EXPIRATION
//done on page load
//fills in a <select> with <options>
$(function() {
	var elem = $('#card-exp-year');
	elem.html("");

	//get current year
	var d = 	new Date()
	var year = 	d.getFullYear();

	//default first value
	elem.append("<option value='0'>Please choose.</option>");

	//options for years
	for (var i = year; i < year + 11; i ++) {
		elem.append("<option value='" + i + "'>" + i + "</option>");
	}

	console.log("Loaded list of expiration years.");
});

//HIDE "THIS YEAR" IF USER CHOOSES AN EXPIRATION MONTH IN THE PAST
//user cannot choose an expiration in a past month for this year
$('#add-card').on('change', '#card-exp-month', function() {
	//get value from month chosen
	var expMonth = 		$(this).val();

	//get current month
	var d = 			new Date();
	var currentMonth = 	d.getMonth() + 1;
	var currentYear = 	d.getFullYear();

	//check if expiration month is in the past
	//hide the option for this year if month is in the past
	if (expMonth < currentMonth) {
		$('#card-exp-year option[value=' + currentYear + ']').css({"display": "none"});
	}
	else {
		$('#card-exp-year option[value=' + currentYear + ']').css({"display": "block"});
	}

	return;
});

//VALIDATE ADD NEW CARD FORM
$('#add-card').submit(function (e) {
	var form = 			$('#add-card');
	var customerId = 	$('#customer-id').val().trim();
	var customerName = 	$('#customer-name').val().trim();
	var cardholder = 	$('#cardholder-name').val().trim();
	var cardNum = 		$('#card-number').val().trim().replace(' ', '').replace('-', '');
	var expYear = 		parseInt($('#card-exp-year').val());
	var expMonth = 		parseInt($('#card-exp-month').val());
	var cvc = 			$('#card-cvc').val().trim();
	var postal = 		$('#card-postal-code').val().trim();
	var cardType = 		Stripe.card.cardType(cardNum);

	//disable the submit button
	//so the user cannot add the same card twice by mistake
	$('#add-card #submit').prop("disabled", true).text("Adding Card...");

	//hide any existing warnings
	$('#error-msg').html('');

	//make sure each input is valid
	//customer name
	if (customerName.length < 2) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'You must provide a customer name.  This can be the same as the cardholder or the name of a company.  This is used to lookup cards when you want to create a charge.', form);
		return false;
	}

	//cardholder name
	if (cardholder.length < 2) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'Please provide the name of the cardholder as it is given on the card.', form);
		return false;
	}

	//card number
	var cardNumLength = cardNum.length;
	if (cardNumLength < 15 || cardNumLength > 16) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'The card number you provided is ' + cardNumLength + ' digits long, however, it must be exactly 15 or 16 digits.', form);
		return false;
	}
	if (Stripe.card.validateCardNumber(cardNum) === false) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'The card number you provided is not valid.', form);
		return false;
	}

	//expiration
	var d = 		new Date();
	var nowMonth = 	d.getMonth() + 1;
	var nowYear = 	d.getFullYear();
	//month
	if (expMonth === 0 || expMonth === '0') {
		e.preventDefault();
		showFormErrorMsg('Error!', 'Please choose the card\'s expiration month.', form);
		return false;
	}
	//year
	if (expYear === 0 || expYear === '0') {
		e.preventDefault();
		showFormErrorMsg('Error!', 'Please choose the card\'s expiration year.', form);
		return false;
	}
	//both
	if (expYear === nowYear && expMonth < nowMonth) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'The card\'s expiration must be in the future.', form);
		return false;
	}
	if (Stripe.card.validateExpiry(expMonth, expYear) === false) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'The card\'s expiration must be in the future.', form);
		return false;
	}

	//cvc
	if (Stripe.card.validateCVC(cvc) === false) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'The security code you provided is invalid.', form);
		return false;
	}
	if (cardType === "American Express" && cvc.length !== 4) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'You provided an American Express card but your security code is invalid. The security code must be exactly 4 numbers long.', form);
		return false;
	}
	if (cardType !== "American Express" && cvc.length !== 3) {
		e.preventDefault();
		console.log("asd");

		showFormErrorMsg('Error!', 'You provided an ' + Stripe.card.cardType(cardNum) + ' card but your security code is invalid. The security code must be exactly 3 numbers long.', form);
		return false;
	}

	//postal code
	if (postal.length < 5 || postal.length > 6) {
		e.preventDefault();
		showFormErrorMsg('Error!', 'The postal code must be exactly 5 numeric or 6 alphanumeric characters.', form);
		return false;
	}

	//create card token
	Stripe.card.createToken({
		name: 			cardholder,
		number: 		cardNum,
		cvc: 			cvc,
		exp_month: 		expMonth,
		exp_year: 		expYear,
		address_zip: 	postal
	}, createTokenCallback)

	function createTokenCallback (status, response) {
		if (response.error) {
			showFormErrorMsg('Error!', 'The credit card could not be saved. Please contact an administrator. Message: ' + response.error.message + '.', form);
			return;
		}

		//perform ajax call
		//save data to db
		//create stripe customer using card token
		$.ajax({
			type: 	"POST",
			url: 	"/card/add/",
			data: {
				customerId: 	customerId,
				customerName: 	customerName,
				cardholder: 	cardholder,
				cardToken: 		response['id'],
				cardExp: 		response['card']['exp_month'] + "/" + response['card']['exp_year'],
				cardLast4: 		response['card']['last4']
			},
			error: function (r) {
				console.log("AJAX save card error");
				console.log(r);
			},
			success: function (r) {
				console.log(r);
			}
		});

		//done
		//show user success panel
		//clear out inputs from add-card form
		//re-enable button to save new cards
		$('#customer-id').val('');
		$('#customer-name').val('');
		$('#cardholder-name').val('');
		$('#card-number').val('');
		$('#card-exp-year').val('0');
		$('#card-exp-month').val('0');
		$('#card-cvc').val('');
		$('#card-postal-code').val('');
		$('#add-card #submit').prop("disabled", false).text("Add Card");
		return;
	}


	//stop form from submitting since it won't do anything anyway
	e.preventDefault();
	return false;
});