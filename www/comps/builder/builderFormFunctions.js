import { getDependentModules } from '../shared/builder.js';
export {MyBuilderFormFunctions as default};

let MyBuilderFormFunction = {
	name:'my-builder-form-function',
	template:`<tr class="builder-form-function">
		<td>
			<img class="dragAnchor" src="images/drag.png" />
		</td>
		<td>
			<div class="row centered gap">
				<my-bool
					v-model="eventBefore"
					:caption0="capGen.after"
					:caption1="capGen.before"
				/>
				<select v-model="event">
					<option value="open"  >{{ capApp.option.open   }}</option>
					<option value="save"  >{{ capApp.option.save   }}</option>
					<option value="delete">{{ capApp.option.delete }}</option>
				</select>
				<my-button image="question.png"
					:active="jsFunctionId !== null"
					@trigger="showHelp"
				/>
			</div>
		</td>
		<td>
			<div class="row gap">
				<select v-model="jsFunctionId">
					<option value="">-</option>
					<option v-for="f in module.jsFunctions.filter(v => v.formId === null || v.formId === formId)"
						:value="f.id"
					>{{ f.name }}</option>
					<optgroup v-for="mod in getDependentModules(module).filter(v => v.id !== module.id && v.jsFunctions.length !== 0)"
						:label="mod.name"
					>
						<option v-for="f in mod.jsFunctions.filter(v => v.formId === null || v.formId === formId)"
							:value="f.id"
						>{{ f.name }}</option>
					</optgroup>
				</select>
				<my-button image="add.png"
					v-if="jsFunctionId === ''"
					@trigger="$emit('createNew','jsFunction',{formId:formId})"
					:captionTitle="capGen.button.create"
				/>
				<my-button image="open.png"
					v-if="jsFunctionId !== ''"
					@trigger="$router.push('/builder/js-function/'+jsFunctionId)"
					:captionTitle="capGen.button.open"
				/>
			</div>
		</td>
		<td>
			<my-button image="delete.png"
				@trigger="$emit('remove')"
				:cancel="true"
				:captionTitle="capGen.button.delete"
			/>
		</td>
	</tr>`,
	props:{
		formId:    { type:String, required:true },
		modelValue:{ type:Object, required:true }
	},
	emits:['createNew','remove','update:modelValue'],
	computed:{
		// inputs
		event:{
			get()  { return this.modelValue.event; },
			set(v) { this.update('event',v); }
		},
		eventBefore:{
			get()  { return this.modelValue.eventBefore; },
			set(v) { this.update('eventBefore',v); }
		},
		jsFunctionId:{
			get()  { return this.modelValue.jsFunctionId; },
			set(v) { this.update('jsFunctionId',v); }
		},
		
		// store
		module:         (s) => s.moduleIdMap[s.formIdMap[s.formId].moduleId],
		moduleIdMap:    (s) => s.$store.getters['schema/moduleIdMap'],
		formIdMap:      (s) => s.$store.getters['schema/formIdMap'],
		jsFunctionIdMap:(s) => s.$store.getters['schema/jsFunctionIdMap'],
		capApp:         (s) => s.$store.getters.captions.builder.form.functions,
		capGen:         (s) => s.$store.getters.captions.generic
	},
	methods:{
		// externals
		getDependentModules,
		
		// actions
		showHelp() {
			let msg = '';
			switch(this.event) {
				case 'open':   msg = this.eventBefore ? this.capApp.help.formLoadedBefore    : this.capApp.help.formLoadedAfter;    break;
				case 'delete': msg = this.eventBefore ? this.capApp.help.recordDeletedBefore : this.capApp.help.recordDeletedAfter; break;
				case 'save':   msg = this.eventBefore ? this.capApp.help.recordSavedBefore   : this.capApp.help.recordSavedAfter;   break;
			}
			this.$store.commit('dialog',{ captionBody:msg, captionTop:this.capGen.contextHelp });
		},
		update(name,value) {
			let v = JSON.parse(JSON.stringify(this.modelValue));
			v[name] = value;
			this.$emit('update:modelValue',v);
		}
	}
};

let MyBuilderFormFunctions = {
	name:'my-builder-form-functions',
	components:{ MyBuilderFormFunction },
	template:`<div class="builder-form-functions">
		<div>
			<my-button image="add.png"
				@trigger="add"
				:caption="capGen.button.add"
			/>
		</div>
		<table class="default-inputs" v-if="modelValue.length !== 0">
			<thead>
				<tr>
					<th></th>
					<th>{{ capApp.event }}</th>
					<th>{{ capApp.jsFunctionId }}*</th>
				</tr>
			</thead>
			<draggable handle=".dragAnchor" tag="tbody" group="functions" itemKey="id" animation="100"
				:fallbackOnBody="true"
				:list="modelValue"
			>
				<template #item="{element,index}">
					<my-builder-form-function
						@createNew="(...args) => $emit('createNew',...args)"
						@remove="remove(index)"
						@update:modelValue="update(index,$event)"
						:formId="formId"
						:key="index"
						:modelValue="element"
					/>
				</template>
			</draggable>
		</table>
		
		<div v-if="anyWithoutFunction" class="warning">
			<img src="images/warning.png" />
			<span>{{ capApp.warning.noJsFunction }}</span>
		</div>
	</div>`,
	props:{
		formId:    { type:String, required:true },
		modelValue:{ type:Array,  required:true }
	},
	emits:['createNew','update:modelValue'],
	computed:{
		anyWithoutFunction:(s) => {
			for(const f of s.modelValue) {
				if(f.jsFunctionId === '')
					return true;
			}
			return false;
		},
		
		// stores
		capApp:(s) => s.$store.getters.captions.builder.form.functions,
		capGen:(s) => s.$store.getters.captions.generic
	},
	methods:{
		// actions
		add() {
			let v = JSON.parse(JSON.stringify(this.modelValue));
			v.unshift({
				event:'open',
				eventBefore:false,
				jsFunctionId:''
			});
			this.$emit('update:modelValue',v);
		},
		remove(i) {
			let v = JSON.parse(JSON.stringify(this.modelValue));
			v.splice(i,1);
			this.$emit('update:modelValue',v);
		},
		update(i,value) {
			let v = JSON.parse(JSON.stringify(this.modelValue));
			v[i] = value;
			this.$emit('update:modelValue',v);
		}
	}
};