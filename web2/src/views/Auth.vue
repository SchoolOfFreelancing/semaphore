<template>
  <div class="auth">
    <v-container
      fluid
      fill-height
      align-center
      justify-center
      class="pa-0"
    >
      <v-form
        ref="signInForm"
        lazy-validation
        v-model="signInFormValid"
        style="width: 300px; height: 300px;"
      >
        <h3 class="text-center mb-8">SEMAPHORE</h3>

        <v-alert
          :value="signInError != null"
          color="error"
          style="margin-bottom: 20px;"
        >{{ signInError }}</v-alert>

        <v-text-field
          v-model="username"
          label="Username"
          :rules="usernameRules"
          autofocus
          required
          :disabled="signInProcess"
        ></v-text-field>

        <v-text-field
          v-model="password"
          label="Password"
          :rules="[v => !!v || 'Password is required']"
          type="password"
          required
          :disabled="signInProcess"
          @keyup.enter.native="signIn"
          style="margin-bottom: 20px;"
        ></v-text-field>
        <v-btn
          color="primary"
          @click="signIn"
          :disabled="signInProcess"
          block
        >
          Sign In
        </v-btn>
      </v-form>
    </v-container>
  </div>
</template>
<style lang="scss">
.auth {
  height: 100vh;
}
</style>
<script>
import axios from 'axios';
import { getErrorMessage } from '@/lib/error';

export default {
  data() {
    return {
      signInFormValid: false,
      signInError: null,
      signInProcess: false,

      password: '',
      username: '',
      email: '',

      emailRules: [
        (v) => !!v || 'Email is required',
      ],
      passwordRules: [
        (v) => !!v || 'Password is required',
        (v) => v.length >= 6 || 'Password too short. Min 6 characters',
      ],
      usernameRules: [
        (v) => !!v || 'Username is required',
      ],
    };
  },

  async created() {
    if (this.isAuthenticated()) {
      document.location = document.baseURI;
    }
  },

  methods: {
    isAuthenticated() {
      return document.cookie.includes('semaphore=');
    },

    async signIn() {
      this.signInError = null;

      if (!this.$refs.signInForm.validate()) {
        return;
      }

      this.signInProcess = true;
      try {
        await axios({
          method: 'post',
          url: '/api/auth/login',
          responseType: 'json',
          data: {
            auth: this.username,
            password: this.password,
          },
        });

        document.location = document.baseURI;
      } catch (err) {
        console.log(err);
        if (err.response.status === 401) {
          this.signInError = 'Incorrect login or password';
        } else {
          this.signInError = getErrorMessage(err);
        }
      } finally {
        this.signInProcess = false;
      }
    },
  },
};
</script>
