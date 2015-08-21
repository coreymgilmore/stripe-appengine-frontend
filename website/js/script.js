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
if (window.location.pathname !== "/") {
	Stripe.setPublishableKey('pk_test_pKzD1QYPWJNrwuGTZ2k0HEkn');
}

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
//true if input is a valid email
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
//parentElem is the parent html element where the <div .error-msg> is located
function showPanelMessage (msg, type, elem) {
	elem.html('<div class="alert alert-' + type +'">' + msg + '</div>');
	return;
}

//SHOW MESSAGES IN MODALS
function showModalMessage (msg, type, elem) {
	elem.html('<div class="alert alert-' + type + '">' + msg + '</div>');
	return
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
	var msg = 	$('#create-init-admin .msg');

	//check if passwords match
	if (doWordsMatch(pass1, pass2) === false) {
		e.preventDefault();
		showPanelMessage("The passwords do not match.", 'danger', msg);
		return false;
	}

	//make sure the password is long enough
	if (isLongPassword(pass1) === false) {
		e.preventDefault();
		showPanelMessage("Error", "Your password is too short. It must be at least " + MIN_PASSWORD_LENGTH + " characters.", 'danger', msg);
		return false;
	}

	//make sure the passwords are not very easy to guess
	if (isSimplePassword(pass1) === true) {
		e.preventDefault();
		showPanelMessage("Error", "The password you provided is too simple. Please choose a better password.", 'danger', msg);
		return false;
	}

	//passwords validated
	//form will submit and save admin user
	//user will see a success/error page
	//then user can log in on main login page
});

//*******************************************************************************
//ADD USER

//ADD A NEW USER
//validate user data and save user via ajax call
$('#form-new-user').submit(function (e) {
	//gather inputs
	var username = 		$('#form-new-user .username').val();
	var password1 = 	$('#form-new-user .password1').val();
	var password2 = 	$('#form-new-user .password2').val();
	var addCards = 		$('#form-new-user .can-add-cards input:checked').val();
	var removeCards = 	$('#form-new-user .can-remove-cards input:checked').val();
	var chargeCards = 	$('#form-new-user .can-charge-cards input:checked').val();
	var reports = 		$('#form-new-user .can-view-reports input:checked').val();
	var admin = 		$('#form-new-user .is-admin input:checked').val();
	var active = 		$('#form-new-user .is-active input:checked').val();
	var msgElem = 		$('#form-new-user .msg');
	var submit = 		$('#form-new-user-submit');

	//check if username is an email
	if (validateEmail(username) === false) {
		e.preventDefault();
		showModalMessage("You must provide an email address as a username.", "danger", msgElem);
		return false;
	}

	//validate password
	//check if passwords match
	if (doWordsMatch(password1, password2) === false) {
		e.preventDefault();
		showModalMessage("The passwords do not match.", "danger", msgElem);
		return false;
	}

	//make sure the password is long enough
	if (isLongPassword(password1) === false) {
		e.preventDefault();
		showModalMessage("Your password is too short. It must be at least " + MIN_PASSWORD_LENGTH + " characters.", "danger", msgElem);
		return false;
	}

	//make sure the passwords are not very easy to guess
	if (isSimplePassword(password1) === true) {
		e.preventDefault();
		showModalMessage("Your password too simple. Choose a more complex password.", "danger", msgElem);
		return false;
	}

	//clear any alerts
	msgElem.html('');

	//add user via ajax call
	e.preventDefault();
	$.ajax({
		type: 	"POST",
		url: 	"/users/add/",
		data: {
			username: 		username,
			password1: 		password1,
			password2: 		password2,
			addCards: 		addCards,
			removeCards: 	removeCards,
			chargeCards: 	chargeCards,
			reports: 		reports,
			admin: 			admin,
			active: 		active
		},
		beforeSend: function() {
			//disable add user btn and show working message
			submit.attr("disabled", true);
			showModalMessage("Saving user...", "info", msgElem);
			return;
		},
		error: function (r) {
			var j = JSON.parse(r['responseText']);
			if (j['ok'] === false) {
				showModalMessage(j['data']['error_msg'], 'danger', msgElem);
				console.log(j);
				return;
			}

			submit.attr("disabled", true);
			return;
		},
		success: function (r) {
			showModalMessage("New user was saved sucessfully!", "success", msgElem);

			//reset inputs
			//hide success message
			setTimeout(function() {
				submit.attr("disabled", false);
				resetAddUserModal();
			}, 3000);
		}
	});
	return false;
});

//RESET NEW USER MODAL TO DEFAULT VALUES
function resetAddUserModal() {
	$('#form-new-user .username').val('');
	$('#form-new-user .password1').val('');
	$('#form-new-user .password2').val('');
	$('#form-new-user .default').attr("checked", true).parent('label').addClass('active').siblings('label').removeClass('active')
	$('.msg').html('');
	return;
}

//RESET ADD NEW USER MODAL IF THE MODAL CLOSES
$('#modal-new-user').on('hidden.bs.modal', function() {
	resetAddUserModal();
	return;
});

//*******************************************************************************
//CHANGE USER PASSWORD

//GET LIST OF USERS WHEN MODALS OPEN
$('#modal-change-pwd, #modal-update-user').on('show.bs.modal', function() {
	getUsers()
	return;
});

//GET LIST OF USERNAMES AND IDS
//fill in the drop downs for editing users and changing passwords
function getUsers() {
	//there are two of these selects (change pwd & update user)
	var userList = $('.user-list');

	$.ajax({
		type: 	"GET",
		url: 	"/users/get/all/",
		dataType: "json",
		beforeSend: function() {
			userList.html('<option value="0">Loading...</option>').attr('disabled', true);
			return;
		},
		error: function (r) {
			userList.html('<option value="0">Error (please see dev tools)</option>');
			console.log(r);
			return;
		},
		success: function (r) {
			//clear options
			userList.html('');
			userList.append("<option value='0'>Please choose...</option>").attr('disabled', false);

			//display list of users in selects
			//do not show 'administrator' user in list so it cannot be updates
			var users = r['data'];
			users.forEach(function (u, index) {
				if (u['username'] === "administrator") {
					return;
				}

				userList.append('<option value="' + u['id'] + '">' + u['username'] + '</option>');
				return;
			});

			return;
		}
	});
}

//CHANGE A USERS PASSWORD
//validate before submitting ajax
$('#form-change-pwd').submit(function (e) {
	//get values
	var id = 		$('#form-change-pwd .user-list').val();
	var pass1 = 	$('#form-change-pwd .password1').val();
	var pass2 = 	$('#form-change-pwd .password2').val();
	var msgElem = 	$('#form-change-pwd .msg');
	var submit = 	$('#change-password-submit');

	//validate password
	//check if passwords match
	if (doWordsMatch(pass1, pass2) === false) {
		e.preventDefault();
		showModalMessage("The passwords do not match.", "danger", msgElem);
		return false;
	}

	//make sure the password is long enough
	if (isLongPassword(pass1) === false) {
		e.preventDefault();
		showModalMessage("Your password is too short. It must be at least " + MIN_PASSWORD_LENGTH + " characters.", "danger", msgElem);
		return false;
	}

	//make sure the passwords are not very easy to guess
	if (isSimplePassword(pass1) === true) {
		e.preventDefault();
		showModalMessage("Your password too simple. Choose a more complex password.", "danger", msgElem);
		return false;
	}

	//ajax to update db
	$.ajax({
		type: 	"POST",
		url: 	"/users/change-pwd/",
		data: {
			userId: id,
			pass1: 	pass1,
			pass2: 	pass2
		},
		beforeSend: function () {
			submit.attr("disabled", true);
			showModalMessage("Saving new password...", "info", msgElem);
			return;
		},
		error: function (r) {
			showModalMessage("An error occured while trying to update this user's password.", "danger", msgElem);
			console.log(r);
			return;
		},
		success: function (r) {
			showModalMessage("This user's password has been updated.", "success", msgElem);

			//reset inputs
			//hide success message
			setTimeout(function() {
				submit.attr("disabled", false);
				resetChangePwdModal();
			}, 3000);
		}
	});

	e.preventDefault();
	return false;
});

//RESET CHANGE PASSWORD MODAL TO DEFAULT VALUES
function resetChangePwdModal() {
	$('.user-list').val('0');
	$('#form-change-pwd .password1').val('');
	$('#form-change-pwd .password2').val('');
	$('.msg').html('');
	return;
}

//RESET CHANGE USER MODAL IF THE MODAL CLOSES
$('#modal-change-pwd').on('hidden.bs.modal', function() {
	resetAddUserModal();
	return;
});

//*******************************************************************************
//CHANGE USER PERMISSIONS

//RESET UPDATE USER MODAL TO DEFAULT VALUES
function resetUpdateUserModal() {
	$('#form-update-user label.btn').attr('disabled', true).removeClass('active');
	$('#form-update-user input[type=radio]').attr('disabled', true).attr('checked', false);
	$('.msg').html('');
	$('#update-user-submit').attr('disabled', true);
	return;
}

//RESET UPDATE USER MODAL IF THE MODAL CLOSES
$('#modal-update-user').on('hidden.bs.modal', function() {
	resetUpdateUserModal();
	return;
});

//GET USER DATA ON SELECTION
//set access control toggles to user's current permissions
//if default user is chosen, 
$('#form-update-user').on('change', '.user-list', function() {
	//get user id from select value
	var userId = 	$(this).val();
	var msgElem = 	$('#form-update-user .msg');

	//if user choosed default option, disable everything
	if (userId === 0) {
		resetUpdateUserModal();
		return;
	}

	//get user data from ajax
	$.ajax({
		type: 	"GET",
		url: 	"/users/get/",
		data: 	{
			userId: userId
		},
		beforeSend: function() {
			resetUpdateUserModal();
			showModalMessage("Retrieving user's permissions...", "info", msgElem);
			return;
		},
		error: function(r) {
			showModalMessage("An error occured while trying to retrieve this users data. Please try again.", "danger", msgElem);
			console.log(r);
			return;
		},
		dataType: "json",
		success: function (j) {
			//hide the alert msg
			msgElem.html('');

			//enable access control toggles
			//enable save btn
			//select the right option again since the entire form was reset in beforeSend
			$('#form-update-user label.btn').attr('disabled', false);
			$('#form-update-user input[type=radio]').attr('disabled', false);
			$('#update-user-submit').attr('disabled', false);

			//make the toggles reflect the user's permissions
			var data = j['data']
			if (data['add_cards']) {
				$('#form-update-user .can-add-cards input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .can-add-cards input[value=false]').attr('checked', true).parent().addClass('active');
			}

			if (data['remove_cards']) {
				$('#form-update-user .can-remove-cards input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .can-remove-cards input[value=false]').attr('checked', true).parent().addClass('active');
			}

			if (data['charge_cards']) {
				$('#form-update-user .can-charge-cards input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .can-charge-cards input[value=false]').attr('checked', true).parent().addClass('active');
			}

			if (data['view_reports']) {
				$('#form-update-user .can-view-reports input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .can-view-reports input[value=false]').attr('checked', true).parent().addClass('active');
			}

			if (data['is_admin']) {
				$('#form-update-user .is-admin input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .is-admin input[value=false]').attr('checked', true).parent().addClass('active');
			}

			if (data['is_active']) {
				$('#form-update-user .is-active input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .is-active input[value=false]').attr('checked', true).parent().addClass('active');
			}

			return;
		}
	});

	return;
});

//SAVE UPDATED USER PERMISSIONS
$('#form-update-user').submit(function (e) {
	var userId = 		$('#form-update-user .user-list').val();
	var addCards = 		$('#form-update-user .can-add-cards label.active input').val();
	var removeCards = 	$('#form-update-user .can-remove-cards label.active input').val();
	var chargeCards = 	$('#form-update-user .can-charge-cards label.active input').val();
	var reports = 		$('#form-update-user .can-view-reports label.active input').val();
	var admin = 		$('#form-update-user .is-admin label.active input').val();
	var active = 		$('#form-update-user .is-active label.active input').val();
	var msgElem = 		$('#form-update-user .msg');
	var submit = 		$('#update-user-submit');

	//quick validation
	if (userId.length === 0) {
		e.preventDefault();
		showModalMessage("A user must be chosen first.", "danger", msgElem);
		return;
	}

	//stop form submission
	e.preventDefault();

	//update user via ajax
	$.ajax({
		type: 	"POST",
		url: 	"/users/update/",
		data: {
			userId: 		userId,
			addCards: 		addCards,
			removeCards: 	removeCards,
			chargeCards: 	chargeCards,
			reports: 		reports,
			admin: 			admin,
			active: 		active
		},
		beforeSend: function() {
			//disable save button
			//show message
			submit.attr('disabled', true);
			showModalMessage("Saving updated permissions...", "info", msgElem);
			return;
		},
		error: function (r) {
			var j = JSON.parse(r['responseText']);
			if (j['ok'] === false) {
				showModalMessage(j['data']['error_msg'], 'danger', msgElem);
				console.log(j);
				return;
			}
			return;
		},
		dataType: "json",
		success: function (j) {
			//user updated successfully
			//show success message
			//re-enable save btn
			showModalMessage("User updated successfully!", "success", msgElem);
			setTimeout(function() {
				submit.attr('disabled', false);
				msgElem.html('');
			}, 3000);

			return;
		}
	});

	return false;
});

//*******************************************************************************
//ADD A NEW CARD

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

//ADD A NEW CREDIT CARD
//validate the card data and save the card via ajax call
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
	var submitBtn = 	$('#add-card .submit-form-btn');
	var msg = 			$('#add-card .msg');

	//hide any existing warnings
	msg.html('');	

	//make sure each input is valid
	//customer name
	if (customerName.length < 2) {
		e.preventDefault();
		showPanelMessage('You must provide a customer name. This can be the same as the cardholder or the name of a company. This is used to lookup cards when you want to create a charge.', "danger", msg);
		return false;
	}

	//cardholder name
	if (cardholder.length < 2) {
		e.preventDefault();
		showPanelMessage('Please provide the name of the cardholder as it is given on the card.', 'danger', msg);
		return false;
	}

	//card number
	var cardNumLength = cardNum.length;
	if (cardNumLength < 15 || cardNumLength > 16) {
		e.preventDefault();
		showPanelMessage('The card number you provided is ' + cardNumLength + ' digits long, however, it must be exactly 15 or 16 digits.', 'danger', msg);
		return false;
	}
	if (Stripe.card.validateCardNumber(cardNum) === false) {
		e.preventDefault();
		showPanelMessage('The card number you provided is not valid.', 'danger', msg);
		return false;
	}

	//expiration
	var d = 		new Date();
	var nowMonth = 	d.getMonth() + 1;
	var nowYear = 	d.getFullYear();
	//month
	if (expMonth === 0 || expMonth === '0') {
		e.preventDefault();
		showPanelMessage('Please choose the card\'s expiration month.', 'danger', msg);
		return false;
	}
	//year
	if (expYear === 0 || expYear === '0') {
		e.preventDefault();
		showPanelMessage('Please choose the card\'s expiration year.', 'danger', msg);
		return false;
	}
	//both
	if (expYear === nowYear && expMonth < nowMonth) {
		e.preventDefault();
		showPanelMessage('The card\'s expiration must be in the future.', 'danger', msg);
		return false;
	}
	if (Stripe.card.validateExpiry(expMonth, expYear) === false) {
		e.preventDefault();
		showPanelMessage('The card\'s expiration must be in the future.', 'danger', msg);
		return false;
	}

	//cvc
	if (Stripe.card.validateCVC(cvc) === false) {
		e.preventDefault();
		showPanelMessage('The security code you provided is invalid.', 'danger', msg);
		return false;
	}
	if (cardType === "American Express" && cvc.length !== 4) {
		e.preventDefault();
		showPanelMessage('You provided an American Express card but your security code is invalid. The security code must be exactly 4 numbers long.', 'danger', msg);
		return false;
	}
	if (cardType !== "American Express" && cvc.length !== 3) {
		e.preventDefault();
		showPanelMessage('You provided an ' + Stripe.card.cardType(cardNum) + ' card but your security code is invalid. The security code must be exactly 3 numbers long.', 'danger', msg);
		return false;
	}

	//postal code
	if (postal.length < 5 || postal.length > 6) {
		e.preventDefault();
		showPanelMessage('The postal code must be exactly 5 numeric or 6 alphanumeric characters.', 'danger', msg);
		return false;
	}

	//disable the submit button so the user cannot add the same card twice by mistake
	submitBtn.prop("disabled", true);
	
	//clear any error messages
	//show "adding card" message
	showPanelMessage('Saving card...', 'info', msg);

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
			showPanelMessage('The credit card could not be saved. Please contact an administrator. Message: ' + response.error.message + '.', 'danger', msg);
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
				var j = JSON.parse(r['responseText']);
				console.log(j);

				if (j['ok'] == false) {
					showPanelMessage(j['data']['error_msg'], 'danger', msg);
					submitBtn.prop("disabled", false).text("Add Card");
					return
				}
				return;
			},
			success: function (r) {
				//card was added successfully
				//clear all inputs
				//show success alert
				resetAddCardPanel();
				showPanelMessage("Card was saved!", 'success', msg);

				//clear the message
				//reenable the add card button
				setTimeout(function() {
					msg.html('');
					submitBtn.prop("disabled", false).text("Add Card");
				}, 3000);

				//reload list of cards
				getCards();

				return;
			}
		});

		return;
	}

	//stop form from submitting since it won't do anything anyway
	e.preventDefault();
	return false;
});

//CLEAR THE ADD CARD FORM
//reset inputs to defaults
function resetAddCardPanel() {
	$('#customer-id').val('');
	$('#customer-name').val('');
	$('#cardholder-name').val('');
	$('#card-number').val('');
	$('#card-exp-year').val('0');
	$('#card-exp-month').val('0');
	$('#card-cvc').val('');
	$('#card-postal-code').val('');
	return;
}

//CLEAR THE FORM WHEN THE USER CLICKS THE CLEAR BTN
$('#add-card').on('click', '.clear-form-btn', function() {
	resetAddCardPanel();
	return;
});

//*******************************************************************************
//REMOVE A CARD

//GET LIST OF CARDS
function getCards() {
	var customerList = $('#customer-list');

	$.ajax({
		type: 	"GET",
		url: 	"/card/get/all/",
		beforeSend: function() {
			customerList.html('<option value="Loading...">');
			return;
		},
		error: function (r) {
			console.log(r);
			console.log(JSON.parse(r['responseText']));
			customerList.html('<option value="Could Not Load">');
			return;
		},
		dataType: "json",
		success: function (j) {
			//put results in data list
			var data = j['data'];
			customerList.html('');
			data.forEach(function (elem, index) {
				var name = 	elem['customer_name'];
				var id = 	elem['id'];

				customerList.append('<option value="' + name + '" data-id="' + id + '">');
			});

			return;
		}
	});
}

//LOAD LIST OF CARDS ON PAGE LOAD
//so user does not have to wait for them to load
$(function() {
	console.log("Loading list of cards...");
	getCards();
});

//GET VALUE OF CARD SELECTED FROM INPUT AUTOCOMPLETE LIST
//gets the id from the data- attribute of the selected option in the datalist
function getCardIdFromDataList(autocompleteElement) {
	var selectedOptionValue = 	autocompleteElement.val();
	var options = 				$('#customer-list option');
	var id = 					"";

	options.each(function() {
		var elemValue = $(this).val();
		var elemId = 	$(this).data('id');

		if (selectedOptionValue === elemValue) {
			id = elemId;
			return false;
		}
	});

	return id;
}

//REMOVE A CARD
$('#remove-card').submit(function (e) {
	//get value of autocomplete list
	var input = 	$('#remove-card .customer-name');
	var custName = 	input.val();
	var custId = 	getCardIdFromDataList(input);

	//btn and alerts
	var btn = 		$('#remove-card .submit-form-btn');
	var msg = 		$('#remove-card .msg');

	//quick validation
	if (custId === 0 || custId === "0" || custId.length === 0) {
		e.preventDefault();
		showPanelMessage("You must choose a customer.", "danger", msg);
		console.log("input val: " + input.val());
		console.log("cust id: " + custId);
		console.log("cust id length: " + custId.length);
		return
	}

	//remove card
	$.ajax({
		type: 	"POST",
		url: 	"/card/remove/",
		data: {
			customerId: 	custId,
			customerName: 	custName
		},
		beforeSend: function() {
			//disable the submit btn and show a message
			btn.prop('disabled', true);
			showPanelMessage('Removing card...', 'info', msg);
			return;
		},
		error: function (r) {
			var j = JSON.parse(r['responseText']);
			if (j['ok'] === false) {
				btn.prop('disabled', false);
				showPanelMessage('An error occured while removing this card. Please contact an administrator.', 'danger', msg);
				console.log(j);
				return;
			}
		},
		dataType: 'json',
		success: function (j) {
			//card was removed
			//show success message
			btn.prop('disabled', false);
			showPanelMessage('Card was removed!', 'success', msg);

			//clear the chosen option
			//reload list of cards
			input.val('');
			getCards();

			setTimeout(function() {
				msg.html('');
			}, 3000);

			return;
		}
	});

	e.preventDefault();
	return false;
});

//*******************************************************************************
//CHARGE A CARD

//LOAD DATA INTO THE PANEL WHEN A CUSTOMER IS CHOSEN
//typed in or selected from the drop down menu
$('#charge-card').on('change', '.customer-name', function() {
	//get value of autocomplete list
	var input = 	$('#charge-card .customer-name');
	var custId = 	getCardIdFromDataList(input);

	var msg = 		$('#charge-card .msg');

	//get customer card data to fill into panel
	$.ajax({
		type: 	"GET",
		url: 	"/card/get/",
		data: {
			customerId: custId
		},
		beforeSend: function() {
			//show loading in readonly inputs
			$('#charge-card .customer-cardholder').val("Loading...");
			$('#charge-card .card-last-four').val("Loading...");
			$('#charge-card .card-expiration').val("Loading...");
			return;
		},
		error: function(r) {
			showPanelMessage("Error while getting a card's data.", "danger", msg);
			console.log(r);
			return;
		},
		dataType: "json",
		success: function (j) {
			//put data into readonly form inputs
			//so a user can verify card information and see some basic data before charging card
			var data = j['data'];
			$('#charge-card .customer-cardholder').val(data['cardholder_name']);
			$('#charge-card .card-last-four').val(data['card_last4']);
			$('#charge-card .card-expiration').val(data['card_expiration']);

			//enable amount, invoice, po inputs
			$('#charge-card .charge-amount').prop('disabled', false);
			$('#charge-card .charge-invoice').prop('disabled', false);
			$('#charge-card .charge-po').prop('disabled', false);

			return;
		}
	});

	return;
});

//CHARGE A CARD
//validate the amount, invoice, and po inputs
//create charge via ajax to stripe
$('#charge-card').submit(function (e) {
	//gather inputs
	var customerNameInput = $('#charge-card .customer-name');
	var customerName = 		customerNameInput.val();
	var customerId = 		getCardIdFromDataList(customerNameInput);
	var amountElem = 		$('#charge-card .charge-amount');
	var amount = 			parseFloat(amountElem.val());
	var invoiceElem = 		$('#charge-card .charge-invoice');
	var invoice = 			invoiceElem.val();
	var poElem = 			$('#charge-card .charge-po');
	var po = 				poElem.val();
	var msg = 				$('#charge-card .msg');
	var btn = 				$('#charge-card-submit');

	//stop form
	e.preventDefault();

	//validate
	if (amount < 1) {
		e.preventDefault();
		showPanelMessage("You must provide an amount to charge that is greater than $0.50.", "danger", msg);
		return;
	}

	//charge card via ajax
	$.ajax({
		type: 	"POST",
		url: 	"/card/charge/",
		data: {
			customerId: 	customerId,
			customerName: 	customerName,
			amount: 		amount,
			invoice: 		invoice, 
			po: 			po
		},
		beforeSend: function() {
			//disabled the inputs
			customerNameInput.prop('disabled', true);
			amountElem.prop('disabled', true);
			invoiceElem.prop('disabled', true);
			poElem.prop('disabled', true);
			btn.prop('disabled', true);

			//show working message
			showPanelMessage("Charging card...", 'info', msg);
			return;
		},
		error: function (r) {
			var j = JSON.parse(r['responseText']);
			if (j['ok'] === false) {
				showPanelMessage(j['data']['error_msg'], 'danger', msg);
				console.log(j);
				return
			}
		},
		dataType: "json",
		success: function (j) {
			//show success message
			showPanelMessage('Card charged!', 'success', msg);

			//clear inputs
			$('#charge-card .customer-name').val('');
			$('#charge-card .customer-cardholder').val('');
			$('#charge-card .card-last-four').val('');
			$('#charge-card .card-expiration').val('');
			amountElem.val('');
			invoiceElem.val('');
			poElem.val('');

			//hide msg and enable btn
			setTimeout(function() {
				msg.html('');
				btn.prop('disabled', false);
			}, 3000);

			return;
		}
	});

	return false;
});