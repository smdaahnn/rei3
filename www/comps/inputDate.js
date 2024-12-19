import MyCalendarDays         from './calendarDays.js';
import MyCalendarMonth        from './calendarMonth.js';
import {MyCalendarDateSelect} from './calendar.js';
import isDropdownUpwards      from './shared/layout.js';
import {getStringFilled}      from './shared/generic.js';
import {
	getCalendarCutOff0,
	getCalendarCutOff1
} from './shared/calendar.js';
import {
	applyUnixDateToDatetime,
	getDateAtUtcZero,
	getDateShifted,
	getDateFullDayToggled,
	getDateNoUtcZero,
	getUnixFromDate,
	getUnixNowDatetime,
	isUnixUtcZero
} from './shared/time.js';
export {MyInputDate as default};

let MyInputDateEntryInput = {
	name:'my-input-date-entry-input',
	template:`<input data-is-input="1"
		v-model="value"
		@change="$emit('update:modelValue',$event.target.value)"
		@keyup.enter.stop="$emit('update:modelValue',$event.target.value)"
		:disabled="isReadonly"
		:placeholder="caption"
		:style="styles"
	/>`,
	props:{
		caption:   { type:String,  required:true },
		isReadonly:{ type:Boolean, required:true },
		modelValue:{ required:true },
		size:      { type:Number,  required:true }
	},
	emits:['update:modelValue'],
	computed:{
		styles:(s) => `width:${s.size}ch;`,
		value:{
			get()  { return this.modelValue; },
			set(v) {}
		}
	}
};

let MyInputDateEntry = {
	name:'my-input-date-entry',
	components:{MyInputDateEntryInput},
	template:`<div class="entry date-inputs">
		
		<span v-if="captionPrefix !== ''" class="prefix">
			{{ captionPrefix }}
		</span>
	
		<!-- mobile date inputs -->
		<div class="mobile-inputs" v-if="isMobile">
			<input step="1" type="datetime-local" v-if="isDate && isTime" v-model="inputMobileDatetime" :disabled="isReadonly" />
			<input step="1" type="date" v-if="isDate && !isTime" v-model="inputMobileDate" :disabled="isReadonly" />
			<input step="1" type="time" v-if="!isDate && isTime" v-model="inputMobileTime" :disabled="isReadonly" />
		</div>
		
		<!-- non-mobile date inputs -->
		<template v-if="!isMobile && isDate">
			
			<!-- year, month, day - ordered by user setting -->
			<template v-for="i in 5">
				<my-input-date-entry-input
					v-if="i % 2 !== 0"
					@update:modelValue="parseInput(getInputType(i-1),$event)"
					:caption="getInputCaption(i-1)"
					:isReadonly="isReadonly"
					:modelValue="getInputValue(i-1)"
					:size="getInputSize(i-1)"
				/>
				<span v-if="i === 1 || i === 3">{{ inputSeparatorSymbol }}</span>
			</template>
		</template>
		
		<!-- non-mobile time inputs -->
		<template v-if="!isMobile && isTime">
			<span v-if="isDate" class="time-separator"></span>
			
			<my-input-date-entry-input
				@update:modelValue="parseInput('H',$event)"
				:caption="capApp.inputHour"
				:isReadonly="isReadonly"
				:modelValue="inputHour"
				:size="2"
			/>
			<span>:</span>
			<my-input-date-entry-input
				@update:modelValue="parseInput('M',$event)"
				:caption="capApp.inputMinute"
				:isReadonly="isReadonly"
				:modelValue="inputMinute"
				:size="2"
			/>
			<span>:</span>
			<my-input-date-entry-input
				@update:modelValue="parseInput('S',$event)"
				:caption="capApp.inputSecond"
				:isReadonly="isReadonly"
				:modelValue="inputSecond"
				:size="2"
			/>
		</template>
	</div>`,
	props:{
		captionPrefix:{ type:String,  required:true },
		isDate:       { type:Boolean, required:true },
		isTime:       { type:Boolean, required:true },
		isReadonly:   { type:Boolean, required:true },
		modelValue:   { required:true }
	},
	emits:['update:modelValue'],
	computed:{
		localDate() {
			if(this.modelValue === null) return null;
			
			// non-datetime is always handled as UTC
			let d = new Date(this.modelValue * 1000);
			return this.isDateTime ? d : this.getDateShifted(d,true);
		},
		
		// inputs
		inputYear:  (s) => s.localDate === null ? '' : s.localDate.getFullYear(),
		inputMonth: (s) => s.localDate === null ? '' : s.getStringFilled(s.localDate.getMonth()+1,2,'0'),
		inputDay:   (s) => s.localDate === null ? '' : s.getStringFilled(s.localDate.getDate(),2,'0'),
		inputHour:  (s) => s.localDate === null ? '' : s.getStringFilled(s.localDate.getHours(),2,'0'),
		inputMinute:(s) => s.localDate === null ? '' : s.getStringFilled(s.localDate.getMinutes(),2,'0'),
		inputSecond:(s) => s.localDate === null ? '' : s.getStringFilled(s.localDate.getSeconds(),2,'0'),
		
		/* alternative inputs for mobile devices */
		/* uses datetime-local for native datetime inputs on mobile devices */
		/* works reliably with format 2019-12-31T12:12:00 */
		inputMobileDatetime:{
			get() {
				return this.modelValue !== null
					? `${this.inputYear}-${this.inputMonth}-${this.inputDay}T${this.inputHour}:${this.inputMinute}:${this.inputSecond}` : '';
			},
			set(v) {
				let d = new Date(v);
				if(!isNaN(d.getTime()))
					this.$emit('update:modelValue',d.getTime() / 1000);
			}
		},
		inputMobileDate:{
			get() {
				return this.modelValue !== null
					? `${this.inputYear}-${this.inputMonth}-${this.inputDay}` : '';
			},
			set(v) {
				let d = new Date(v);
				if(!isNaN(d.getTime()))
					this.$emit('update:modelValue',d.getTime() / 1000);
			}
		},
		inputMobileTime:{
			get() {
				return this.modelValue !== null
					? `${this.inputHour}:${this.inputMinute}:${this.inputSecond}` : '';
			},
			set(v) {
				let m = v.match(/^(\d+)\:(\d+)\:(\d+)$/);
				if(m !== null && m.length === 4)
					return this.$emit('update:modelValue',
						(parseInt(m[1]) * 60 * 60) + (parseInt(m[2]) * 60) + parseInt(m[3])
					);
				
				// iOS fix: Safari mobile ignores input steps and shows only hours/minutes
				m = v.match(/^(\d+)\:(\d+)$/);
				if(m !== null && m.length === 3)
					return this.$emit('update:modelValue',
						(parseInt(m[1]) * 60 * 60) + (parseInt(m[2]) * 60)
					);
			}
		},
		
		// display
		inputSeparatorSymbol:(s) => {
			if(s.settings.dateFormat.indexOf('/') !== -1) return '/';
			if(s.settings.dateFormat.indexOf('.') !== -1) return '.';
			return '-';
		},
		
		// simple
		isDateOnly:(s) => s.isDate && !s.isTime,
		isDateTime:(s) => s.isDate && s.isTime,
		isTimeOnly:(s) => !s.isDate && s.isTime,
		
		// stores
		capApp:  (s) => s.$store.getters.captions.input.date,
		isMobile:(s) => s.$store.getters.isMobile,
		settings:(s) => s.$store.getters.settings
	},
	methods:{
		// externals
		getDateAtUtcZero,
		getDateShifted,
		getStringFilled,
		
		parseInput(name,input) {
			let d;
			let p = parseInt(input);
			if(isNaN(p)) return;
			
			if(this.modelValue !== null) {
				// valid value, parse from unix time
				d = new Date(this.modelValue * 1000);
			}
			else {
				// empty date: start with current date and time at UTC midnight
				// empty time: start with UTC zero
				d = !this.isTimeOnly ? this.getDateAtUtcZero(new Date()) : new Date(0);
			}
			
			if(this.isTimeOnly) {
				switch(name) {
					case 'H': d.setUTCHours(p);   break;
					case 'M': d.setUTCMinutes(p); break;
					case 'S': d.setUTCSeconds(p); break;
				}
				
				// allowed values: between 0 (00:00:00) and 86399 (23:59:59)
				if(Math.floor(d.getTime() / 1000) > 86399) d = new Date(86399000);
				if(Math.floor(d.getTime() / 1000) < 0)     d = new Date(0);
				
			} else {
				// apply timezone offset for pure dates to correctly apply day input
				if(this.isDateOnly)
					d = this.getDateShifted(d,true);
				
				switch(name) {
					case 'Y': d.setFullYear(p); break;
					case 'm': d.setMonth(p-1);  break;
					case 'd': d.setDate(p);     break;
					case 'H': d.setHours(p);   break;
					case 'M': d.setMinutes(p); break;
					case 'S': d.setSeconds(p); break;
				}
				
				if(this.isDateOnly)
					d = this.getDateShifted(d,false);
			}
			this.$emit('update:modelValue',Math.floor(d.getTime() / 1000));
		},
		getInputCaption(position) {
			switch(this.getInputType(position)) {
				case 'Y': return this.capApp.inputYear;  break;
				case 'm': return this.capApp.inputMonth; break;
				case 'd': return this.capApp.inputDay;   break;
			}
		},
		getInputSize(position) {
			return this.getInputType(position) === 'Y' ? 4 : 2;
		},
		getInputType(position) {
			return this.settings.dateFormat.substr(position,1);
		},
		getInputValue(position) {
			switch(this.getInputType(position)) {
				case 'Y': return this.inputYear;   break;
				case 'm': return this.inputMonth;  break;
				case 'd': return this.inputDay;    break;
				case 'H': return this.inputHour;   break;
				case 'M': return this.inputMinute; break;
			}
		}
	}
};

let MyInputDate = {
	name:'my-input-date',
	components:{
		MyCalendarDateSelect,
		MyCalendarDays,
		MyCalendarMonth,
		MyInputDateEntry
	},
	template:`<div class="input-date"
		@blur="$emit('blurred')"
		@focus="$emit('focused')"
		@keyup.esc="escaped"
		v-click-outside="escaped"
	>
		<div class="content-wrap">
			
			<div class="entries">
				<my-input-date-entry
					v-model="unixFromInput"
					:captionPrefix="isRange ? capApp.dateFrom : ''"
					:isDate="isDate"
					:isTime="isTime && !fullDay"
					:isReadonly="isReadonly"
				/>
				
				<my-input-date-entry
					v-if="isRange"
					v-model="unixToInput"
					:captionPrefix="capApp.dateTo"
					:isDate="isDate"
					:isTime="isTime && !fullDay"
					:isReadonly="isReadonly"
				/>
			</div>
			
			<div v-if="!isReadonly" class="actions">
				<!-- full day selector -->
				<my-button
					v-if="isDate && isTime && isRange"
					@trigger="toggleFullDayRange"
					:captionTitle="capApp.fullDayHint"
					:image="fullDay ? 'clockOff.png' : 'clock.png'"
					:naked="true"
				/>
				
				<!-- toggle calendar -->
				<my-button image="calendar.png"
					v-if="isDate"
					@trigger="toggleCalendar"
					:naked="true"
				/>
				
				<!-- set null -->
				<my-button image="cancel.png"
					@trigger="setNull()"
					:active="unixFrom !== null || unixTo !== null"
					:naked="true"
				/>
			</div>
		</div>
		
		<div class="input-dropdown-wrap" v-if="showCalendar" :class="{ upwards:showUpwards }">
			<div class="input-dropdown" :class="{ upwards:showUpwards }">
				<div class="top lower">
					<div class="area nowrap"></div>
					<div class="area nowrap default-inputs">
						<my-calendar-date-select v-model="date" :daysShow="viewMonth ? 42 : 7" />
					</div>
					<div class="area nowrap">
						<my-button image="arrowsSwitch.png"
							v-if="isDateTime"
							@trigger="viewMonth = !viewMonth"
							:captionTitle="capApp.button.viewHint"
						/>
						<my-button image="calendarDot.png"
							@trigger="goToToday"
							:captionTitle="capApp.button.todayHint"
						/>
					</div>
				</div>
				
				<my-calendar-days
					v-if="!viewMonth"
					@set-date="date = $event"
					@date-selected="dateSet"
					:date="date"
					:date0="date0"
					:date1="date1"
					:dateSelect0="dateSelect0"
					:dateSelect1="dateSelect1"
					:daysShow="7"
					:isInput="true"
					:isRange="isRange"
				/>
				<my-calendar-month
					v-if="viewMonth"
					@set-date="date = $event"
					@date-selected="dateSetByMonthView"
					:date="date"
					:date0="date0"
					:date1="date1"
					:dateSelect0="dateSelect0"
					:dateSelect1="dateSelect1"
					:inputTime="isTime"
					:isInput="true"
					:isRange="isRange"
				/>
			</div>
		</div>
	</div>`,
	props:{
		isDate:    { type:Boolean, required:true },
		isTime:    { type:Boolean, required:true },
		isRange:   { type:Boolean, required:false, default:false },
		isReadonly:{ type:Boolean, required:false, default:false },
		unixFrom:  { required:true },
		unixTo:    { required:false, default:null }
	},
	emits:['blurred','focused','set-unix-from','set-unix-to'],
	data() {
		return {
			date:new Date(),    // date to control calendar navigation
			dateSelect0:null,   // for date range selection, start date
			dateSelect1:null,   // for date range selection, end date
			showUpwards:false,  // show calendar dropdown above input
			showCalendar:false,
			viewMonth:true      // calendar view is either month (true) or days (false)
		};
	},
	mounted() {
		this.viewMonth = this.fullDay;
	},
	computed:{
		// full day events start and end at 00:00:00 UTC
		// if used in datetime context, they can only be used in date range,
		//  otherwise regular datetime value can appear as full day event
		fullDay:(s) => {
			if(s.isDate && !s.isTime) return true;  // pure dates are always full day
			if(!s.isDate && s.isTime) return false; // pure times are never full day
			
			// if not pure date, a range is required to allow for full day events
			// otherwise we cannot separate regular from full day events
			if(!s.isRange) return false;
			
			// full day if start and end date are not set or UTC 00:00:00
			return (s.unixFrom === null || s.isUnixUtcZero(s.unixFrom))
				&& (s.unixTo   === null || s.isUnixUtcZero(s.unixTo));
		},
		
		// date|time inputs
		unixFromInput:{
			get()  { return this.unixFrom; },
			set(v) { this.$emit('set-unix-from',v); }
		},
		unixToInput:{
			get()  { return this.unixTo; },
			set(v) {
				// if to unix is smaller, set to from
				if(v !== null && v < this.unixFromInput)
					v = this.unixFromInput;
				
				this.$emit('set-unix-to',v);
			}
		},
		
		// simple
		isDateTime:(s) => s.isDate && s.isTime,
		
		// start/end date of calendar (month input)
		date0:(s) => s.getCalendarCutOff0((s.viewMonth ? 42 : 7),new Date(s.date.valueOf())),
		date1:(s) => s.getCalendarCutOff1((s.viewMonth ? 42 : 7),new Date(s.date.valueOf()),s.date0),
		
		// stores
		capApp:  (s) => s.$store.getters.captions.input.date,
		isMobile:(s) => s.$store.getters.isMobile
	},
	methods:{
		// externals
		applyUnixDateToDatetime,
		getCalendarCutOff0,
		getCalendarCutOff1,
		getDateAtUtcZero,
		getDateFullDayToggled,
		getDateNoUtcZero,
		getDateShifted,
		getUnixFromDate,
		getUnixNowDatetime,
		isDropdownUpwards,
		isUnixUtcZero,
		
		// events
		escaped() {
			this.showCalendar = false;
		},
		focused() {
			if(!this.isReadonly && !this.showCalendar)
				this.showCalendar = true;
		},
		updateDropdownDirection() {
			let headersPx  = 200; // rough height in px of all headers (menu/form) combined
			let calPx      = 320; // rough height in px of calendar input
			this.showUpwards = this.isDropdownUpwards(this.$el,calPx,headersPx);
		},
		
		// actions
		goToToday() {
			let now = new Date();
			
			const todayIsVisible = (
				this.viewMonth
				&& now.getMonth()    === this.date.getMonth()
				&& now.getFullYear() === this.date.getFullYear()
			) || (
				!this.viewMonth && now >= this.date0 && now <= this.date1
			);
			
			// move view to show 'today'
			if(!todayIsVisible)
				return this.date = now;
			
			// view is already on 'today', apply value
			if(this.viewMonth) {
				const v = this.getUnixFromDate(this.getDateAtUtcZero(now));
				this.dateSetByMonthView(v,v,false);
			} else {
				const v = this.getUnixFromDate(now);
				this.dateSet(v,v);
			}
		},
		toggleCalendar() {
			if(!this.showCalendar) {
				
				// reset date range selection
				if(this.isRange) {
					this.dateSelect0 = this.unixFrom !== null ? new Date(this.unixFrom * 1000) : null;
					this.dateSelect1 = this.unixTo   !== null ? new Date(this.unixTo   * 1000) : null;
				}
				else {
					// no range, use start date for range start/end
					this.dateSelect0 = this.unixFrom !== null ? new Date(this.unixFrom * 1000) : null;
					this.dateSelect1 = this.unixFrom !== null ? new Date(this.unixFrom * 1000) : null;
				}
				
				// fresh selection, start calendar in month of start date
				if(this.unixFrom !== null) {
					this.date = new Date(this.unixFrom * 1000);
				}
				else {
					// if start date is empty, set to now
					this.date = new Date();
					
					if(this.fullDay)
						this.date = this.getDateAtUtcZero(this.date);
				}
				
				// decide dropdown direction
				this.updateDropdownDirection();
			}
			this.showCalendar = !this.showCalendar;
		},
		toggleFullDayRange() {
			// if range is empty, set both dates to a datetime today
			if(this.unixFrom === null && this.unixTo === null) {
				let unix = this.getUnixFromDate(this.getDateNoUtcZero(new Date()));
				this.unixFromInput = unix;
				this.unixToInput   = unix;
				return;
			}
			
			// toggle dates (if set) between date / datetime
			// emit change (instead of setting unixInput)
			// otherwise timing issue exists: unixToInput < unixFromInput
			if(this.unixFrom !== null)
				this.$emit('set-unix-from',this.getUnixFromDate(this.getDateFullDayToggled(
					new Date(this.unixFrom*1000),this.fullDay)));
			
			if(this.unixTo !== null)
				this.$emit('set-unix-to',this.getUnixFromDate(this.getDateFullDayToggled(
					new Date(this.unixTo*1000),this.fullDay)));
		},
		dateSet(unix0,unix1) {
			this.dateSelect0 = new Date(unix0 * 1000);
			this.$emit('set-unix-from',unix0);
			
			if(this.isRange) {
				this.dateSelect1 = new Date(unix1 * 1000);
				this.$emit('set-unix-to',unix1);
			}
			this.showCalendar = false;
		},
		dateSetByMonthView(unix0,unix1,middleClick) {
			if(this.fullDay)
				return this.dateSet(unix0,unix1);
			
			// if not full day values, only apply date component
			let unixFrom = this.unixFromInput;
			let unixTo   = this.unixToInput;
			
			if(unixFrom === null) unixFrom = this.getUnixNowDatetime();
			if(unixTo   === null) unixTo   = this.getUnixNowDatetime();
			
			this.dateSet(
				this.applyUnixDateToDatetime(unixFrom,unix0),
				this.applyUnixDateToDatetime(unixTo,unix1)
			);
		},
		setNull() {
			this.unixFromInput = null;
			this.unixToInput   = null;
			this.showCalendar  = false;
		}
	}
};